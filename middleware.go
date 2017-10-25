package torus

import (
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"net/http"
)

func contains(allowedMethods []string, method string) bool {
	for _, m := range allowedMethods {
		if method == m {
			return true
		}
	}
	return false
}

func RequireMethod(handler http.Handler, allowedMethod string) http.Handler {
	f := func(w http.ResponseWriter, r *http.Request) {
		if allowedMethod != r.Method {
			s := http.StatusMethodNotAllowed
			http.Error(w, http.StatusText(s), s)
			return
		}
		handler.ServeHTTP(w, r)
	}
	return http.HandlerFunc(f)
}

func RequireMethods(handler http.Handler, allowedMethods []string) http.Handler {
	f := func(w http.ResponseWriter, r *http.Request) {
		if !contains(allowedMethods, r.Method) {
			s := http.StatusMethodNotAllowed
			http.Error(w, http.StatusText(s), s)
			return
		}
		handler.ServeHTTP(w, r)
	}
	return http.HandlerFunc(f)
}

type HandlerFuncWithError func(w http.ResponseWriter, r *http.Request) error

func ErrorToInternalServerError(handler HandlerFuncWithError) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := handler(w, r); err != nil {
			c := appengine.NewContext(r)
			log.Errorf(c, "Error getting for verification from datastore: %v", err)
			s := http.StatusInternalServerError
			http.Error(w, http.StatusText(s), s)
		}
	}
}
