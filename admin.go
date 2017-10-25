package torus

import (
	"fmt"
	"net/http"

	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
)

func recomputeScores(w http.ResponseWriter, r *http.Request) error {
	ctx := appengine.NewContext(r)

	q := datastore.NewQuery(QUESTION_KIND)
	var ss []*StoreQuestion
	var err error
	keys, err := q.GetAll(ctx, &ss)
	log.Debugf(ctx, "Got %v questions", len(ss))
	if err != nil {
		return fmt.Errorf("error querying from datastore: %v", err)
	}
	for i, s := range ss {
		ups, err1 := datastore.NewQuery(VOTE_ENTRY_KIND).Ancestor(keys[i]).Filter("Vote = ", up).Count(ctx)
		downs, err2 := datastore.NewQuery(VOTE_ENTRY_KIND).Ancestor(keys[i]).Filter("Vote = ", down).Count(ctx)
		if err1 != nil || err2 != nil {
			return fmt.Errorf("errors encountered getting ups/downs: %v, %v", err1, err2)
		}
		log.Debugf(ctx, "Got %v ups and %v downs", ups, downs)

		s.Score = ups - downs
	}

	if _, err := datastore.PutMulti(ctx, keys, ss); err != nil {
		return fmt.Errorf("error putting updated questions back into datastore: %v", err)
	}

	return nil
}
