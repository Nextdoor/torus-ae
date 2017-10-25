package torus

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/user"
)

const (
	QUESTION_KIND     = "Question"
	VOTE_ENTRY_KIND   = "VoteEntry"
	DIGEST_STATE_KIND = "DigestState"
)

const QUESTION_SUBMISSION_ENABLED = true

type ClientQuestion struct {
	ID         int64     `json:"id"`
	Anonymous  bool      `json:"anonymous"`
	Content    string    `json:"content"`
	AuthorName string    `json:"authorName"`
	Timestamp  time.Time `json:"timestamp"`
	Score      int       `json:"score"`
	MyVote     string    `json:"myVote"`
	MyQuestion bool      `json:"myQuestion"`
}

type ClientQuestionWithClientId struct {
	ClientQuestion
	ClientID int64 `json:"clientId"`
}

type Status struct {
	ID                        string `json:"id"`
	Name                      string `json:"name"`
	LoginURL                  string `json:"loginUrl"`
	LogoutURL                 string `json:"logoutUrl"`
	QuestionSubmissionEnabled bool   `json:"questionSubmissionEnabled"`
}

type VoteEntry struct {
	UserID     string
	QuestionID int64
	Vote       voteDirection
}

type ClientVote struct {
	Vote string `json:"vote"`
}

func init() {
	http.HandleFunc("/v1/submit", ErrorToInternalServerError(submit))
	http.HandleFunc("/v1/list", ErrorToInternalServerError(list))
	http.HandleFunc("/v1/status", ErrorToInternalServerError(status))
	http.Handle("/v1/votes/", RequireMethod(ErrorToInternalServerError(vote), http.MethodPut))
	http.Handle("/v1/question/", RequireMethod(ErrorToInternalServerError(question), http.MethodDelete))
	http.Handle("/v1/status/digest",
		RequireMethods(ErrorToInternalServerError(digest), []string{http.MethodPut, http.MethodDelete}))
	http.HandleFunc("/v1/admin/recompute", ErrorToInternalServerError(recomputeScores))
	http.HandleFunc("/v1/admin/digest", ErrorToInternalServerError(sendDigestsHandler))
}

func question(w http.ResponseWriter, r *http.Request) error {
	ctx := appengine.NewContext(r)
	u := user.Current(ctx)
	if u == nil {
		v := http.StatusUnauthorized
		http.Error(w, http.StatusText(v), v)
		return nil
	}

	_, idStr := path.Split(r.URL.Path)
	id, err := strconv.ParseInt(idStr, 10, 0)
	if err != nil {
		log.Debugf(ctx, "Got unexpected id: %v", idStr)
		v := http.StatusBadRequest
		http.Error(w, http.StatusText(v), v)
		return nil
	}

	if err := deleteQuestion(ctx, id, u.ID); err == ErrForbidden {
		log.Debugf(ctx, "forbidden - %v tried to delete question %v", u.ID, id)
		v := http.StatusForbidden
		http.Error(w, http.StatusText(v), v)
		return nil
	} else if err != nil {
		return fmt.Errorf("couldn't delete question: %v", err)
	}

	w.Write([]byte("{}"))
	return nil
}

func vote(w http.ResponseWriter, r *http.Request) error {
	ctx := appengine.NewContext(r)
	u := user.Current(ctx)
	if u == nil {
		v := http.StatusUnauthorized
		http.Error(w, http.StatusText(v), v)
		return nil
	}

	var v ClientVote
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		return fmt.Errorf("got error parsing vote body: %v", err)
	}

	newVote, err := FromString(v.Vote)
	if err == ErrInvalidString {
		log.Debugf(ctx, "Got unexpected vote direction: %v", v.Vote)
		v := http.StatusBadRequest
		http.Error(w, http.StatusText(v), v)
		return nil
	} else if err != nil {
		return fmt.Errorf("got error parsing vote direction: %v", v.Vote)
	}

	_, idStr := path.Split(r.URL.Path)
	id, err := strconv.ParseInt(idStr, 10, 0)
	if err != nil {
		log.Debugf(ctx, "Got unexpected id: %v", idStr)
		v := http.StatusBadRequest
		http.Error(w, http.StatusText(v), v)
		return nil
	}

	if err := storeVote(ctx, u.ID, newVote, id); err == ErrNoSuchQuestion {
		v := http.StatusBadRequest
		http.Error(w, http.StatusText(v), v)
		return nil
	} else if err != nil {
		return fmt.Errorf("error storing vote: %v", err)
	}

	w.Write([]byte("{}"))
	return nil
}

func status(w http.ResponseWriter, r *http.Request) error {
	ctx := appengine.NewContext(r)
	u := user.Current(ctx)
	s := Status{QuestionSubmissionEnabled: QUESTION_SUBMISSION_ENABLED}
	if u == nil {
		var err error
		if s.LoginURL, err = user.LoginURL(ctx, "/submit/"); err != nil {
			return fmt.Errorf("error getting login URL: %v", err)
		}
	} else {
		var err error
		s.ID = u.ID
		s.Name = u.String()
		if s.LogoutURL, err = user.LogoutURL(ctx, "/"); err != nil {
			return fmt.Errorf("error getting logout URL: %v", err)
		}
	}

	json.NewEncoder(w).Encode(s)
	return nil
}

func submit(w http.ResponseWriter, r *http.Request) error {
	v := http.StatusForbidden
	http.Error(w, http.StatusText(v), v)
	return nil

	ctx := appengine.NewContext(r)
	u := user.Current(ctx)
	if u == nil {
		v := http.StatusUnauthorized
		http.Error(w, http.StatusText(v), v)
		return nil
	}

	if !QUESTION_SUBMISSION_ENABLED {
		v := http.StatusForbidden
		http.Error(w, http.StatusText(v), v)
		return nil
	}

	var x ClientQuestionWithClientId
	json.NewDecoder(r.Body).Decode(&x)

	userString := ""
	if !x.Anonymous {
		userString = u.String()
	}

	y := &StoreQuestion{
		Content:    x.Content,
		Anonymous:  x.Anonymous,
		AuthorID:   u.ID,
		AuthorName: userString,
		Timestamp:  time.Now(),
	}

	key, err := storeQuestion(ctx, y)
	if err != nil {
		return fmt.Errorf("error putting into datastore: %v", err)
	}

	x.ID = key.IntID()
	x.AuthorName = userString

	json.NewEncoder(w).Encode(x)
	return nil
}

func list(w http.ResponseWriter, r *http.Request) error {
	ctx := appengine.NewContext(r)
	u := user.Current(ctx)
	ks, ss, err := getAllQuestions(ctx)
	if err != nil {
		return fmt.Errorf("error getting questions: %v", err)
	}

	vs, err := getMyVotes(ctx, ks)
	if err != nil {
		return fmt.Errorf("error getting votes: %v", err)
	}
	cs := make([]ClientQuestion, len(ks))

	for i, s := range ss {
		c := ClientQuestion{
			ID:         ks[i].IntID(),
			Anonymous:  s.Anonymous,
			Timestamp:  s.Timestamp,
			Content:    s.Content,
			Score:      s.Score,
			MyVote:     vs[i].ToString(),
			MyQuestion: u != nil && u.ID == s.AuthorID,
		}
		if !s.Anonymous {
			c.AuthorName = s.AuthorName
		}
		cs[i] = c
	}

	is := struct {
		Questions []ClientQuestion `json:"questions"`
	}{cs}

	json.NewEncoder(w).Encode(is)
	return nil
}

func digest(w http.ResponseWriter, r *http.Request) error {
	ctx := appengine.NewContext(r)
	u := user.Current(ctx)
	if u == nil {
		v := http.StatusUnauthorized
		http.Error(w, http.StatusText(v), v)
		return nil
	}

	switch r.Method {
	case http.MethodPut:
		return changeDigestState(ctx, u.ID, u.Email, true)
	case http.MethodDelete:
		return changeDigestState(ctx, u.ID, u.Email, false)
	default:
		// other methods should have been filtered out before this one is called
		panic("unexpected method " + r.Method)
	}
}

func sendDigestsHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := appengine.NewContext(r)

	return sendDigests(ctx)
}
