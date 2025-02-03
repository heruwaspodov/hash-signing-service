package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash-signing-service/config"
	"io/ioutil"
	"log/slog"
	"net/http"
	"os"
)

type BodyContentJsonData struct {
	body map[string]interface{}
}

// Logger is a middleware function with a parameter
func Logger(_ *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			jsonHandler := slog.NewJSONHandler(os.Stderr, nil)
			syslog := slog.New(jsonHandler)

			logMessage := fmt.Sprintf("[%s] %s %s %s %s %s",
				r.Method,
				r.RequestURI,
				r.RemoteAddr,
				r.Header,
				GetBody(r),
				r.Response,
			)

			syslog.Info(logMessage)

			next.ServeHTTP(w, r)
		})
	}
}

func GetBody(r *http.Request) any {
	var jsonData map[string]interface{}
	var body any
	// Shallow copy of the request
	copyReq := new(http.Request)
	*copyReq = *r

	// Read request body
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil
	}

	// Copy Body
	copyReq.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

	// Important: Restore the original request Body for further use
	r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

	// Read and manipulate request body copy
	b, err := ioutil.ReadAll(copyReq.Body)
	if err != nil {
		return r.Body
	}

	err = json.Unmarshal(b, &jsonData)

	if err != nil {
		if len(b) > 0 {
			body = string(b)[0]
		} else {
			body = b
		}
	} else {
		content := BodyContentJsonData{
			body: jsonData,
		}

		jsonObject := map[string]interface{}{"body": content.body}
		body, _ = json.Marshal(jsonObject)
	}

	return body
}
