package torus

import (
	"errors"
	"fmt"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/user"
)

var ErrForbidden = errors.New("user is not authorized")
var ErrNoSuchQuestion = errors.New("question ID not valid")

type StoreQuestion struct {
	Anonymous  bool
	Content    string
	AuthorID   string
	AuthorName string
	Timestamp  time.Time
	Score      int
}

type DigestState struct {
	Address       string
	DigestEnabled bool
}

func getAllQuestions(c context.Context) ([]*datastore.Key, []StoreQuestion, error) {
	q := datastore.NewQuery(QUESTION_KIND).Order("-Score")
	var ss []StoreQuestion

	ks, err := q.GetAll(c, &ss)
	if err != nil {
		return nil, nil, fmt.Errorf("error querying from datastore: %v", err)
	}

	return ks, ss, nil
}

func getLatestQuestions(c context.Context) ([]*datastore.Key, []StoreQuestion, error) {
	q := datastore.NewQuery(QUESTION_KIND).
		Filter("Timestamp >=", time.Now().Add(-24*time.Hour))
	var ss []StoreQuestion

	ks, err := q.GetAll(c, &ss)
	if err != nil {
		return nil, nil, fmt.Errorf("error querying from datastore: %v", err)
	}

	return ks, ss, nil
}

func getMyVotes(c context.Context, questionKeys []*datastore.Key) ([]voteDirection, error) {
	u := user.Current(c)
	if u == nil {
		return make([]voteDirection, len(questionKeys)), nil
	}
	uid := u.ID

	keys := make([]*datastore.Key, len(questionKeys))
	for i, id := range questionKeys {
		keyStr := fmt.Sprintf("%v-%v", id.IntID(), uid)
		keys[i] = datastore.NewKey(c, VOTE_ENTRY_KIND, keyStr, 0, questionKeys[i])
	}

	vs := make([]VoteEntry, len(questionKeys))
	vds := make([]voteDirection, len(questionKeys))
	var me appengine.MultiError

	if err := datastore.GetMulti(c, keys, vs); err != nil {
		if multiError, ok := err.(appengine.MultiError); ok {
			me = multiError
		} else {
			return nil, fmt.Errorf("error getting votes: %v", err)
		}
	} else {
		me = make([]error, len(questionKeys))
	}

	for i, v := range vs {
		if me[i] == nil {
			vds[i] = v.Vote
		} else {
			vds[i] = none
			if me[i] != datastore.ErrNoSuchEntity {
				log.Errorf(c, "Error trying to get item for me-vote: %v", keys[i])
			}
		}
	}

	return vds, nil
}

func getSubscribers(c context.Context) ([]string, error) {
	var ss []DigestState
	_, err := datastore.NewQuery(DIGEST_STATE_KIND).
		Filter("DigestEnabled =", true).
		GetAll(c, &ss)

	if err != nil {
		return nil, fmt.Errorf("error getting users with digest enabled: %v", err)
	}

	var addrs = make([]string, len(ss))
	for i := range ss {
		addrs[i] = ss[i].Address
	}
	return addrs, nil
}

func deleteQuestion(c context.Context, qid int64, uid string) error {
	qk := datastore.NewKey(c, QUESTION_KIND, "", qid, nil)

	var s StoreQuestion
	if err := datastore.Get(c, qk, &s); err != nil {
		return fmt.Errorf("error getting for verification from datastore: %v", err)
	} else if s.AuthorID != uid {
		return ErrForbidden
	}

	if err := datastore.Delete(c, qk); err != nil {
		return fmt.Errorf("error deleting from datastore: %v", err)
	}

	keys, err := datastore.NewQuery(VOTE_ENTRY_KIND).Ancestor(qk).KeysOnly().GetAll(c, nil)
	if err != nil {
		return fmt.Errorf("error getting votes for deletion from datastore: %v", err)
	}

	if err := datastore.DeleteMulti(c, keys); err != nil {
		return fmt.Errorf("error deleting votes from datastore: %v", err)
	}

	return nil
}

func storeVote(c context.Context, uid string, vote voteDirection, qid int64) error {
	qk := datastore.NewKey(c, QUESTION_KIND, "", qid, nil)
	vk := datastore.NewKey(c, VOTE_ENTRY_KIND, fmt.Sprintf("%v-%v", qid, uid), 0, qk)

	var (
		s StoreQuestion
		v VoteEntry
	)

	if err := datastore.GetMulti(c, []*datastore.Key{qk, vk}, []interface{}{&s, &v}); err != nil {
		if me, ok := err.(appengine.MultiError); ok {
			if me[0] == datastore.ErrNoSuchEntity {
				log.Debugf(c, "No such entity with ID %v", qid)
				return ErrNoSuchQuestion
			} else if me[0] != nil {
				return fmt.Errorf("error getting question from datastore: %v", err)
			}

			if me[1] == datastore.ErrNoSuchEntity {
				v = VoteEntry{
					UserID:     uid,
					QuestionID: qid,
					Vote:       none,
				}
			} else if me[1] != nil {
				return fmt.Errorf("error getting existing vote: %v", err)
			}
		} else {
			return fmt.Errorf("error getting question or existing vote: %v", err)
		}
	}

	oldVote := v.Vote
	v.UserID = uid
	v.QuestionID = qid
	v.Vote = vote
	s.Score += scoreAdjustment(oldVote, vote)

	if _, err := datastore.PutMulti(c, []*datastore.Key{qk, vk}, []interface{}{&s, &v}); err != nil {
		return fmt.Errorf("error putting question score/vote into datastore: %v", err)
	}

	return nil
}

func storeQuestion(c context.Context, q *StoreQuestion) (*datastore.Key, error) {
	key := datastore.NewIncompleteKey(c, QUESTION_KIND, nil)
	key, err := datastore.Put(c, key, q)

	if err != nil {
		return nil, fmt.Errorf("error putting into datastore: %v", err)
	}

	return key, nil
}

func changeDigestState(c context.Context, uid, email string, digestEnabled bool) error {
	k := datastore.NewKey(c, DIGEST_STATE_KIND, uid, 0, nil)
	_, err := datastore.Put(c, k, &DigestState{email, digestEnabled})

	if err != nil {
		return fmt.Errorf("error putting digest state into datastore: %v", err)
	}

	return nil
}
