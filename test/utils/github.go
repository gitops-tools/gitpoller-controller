/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package utils

import (
	"fmt"
	"net/http"
	"net/http/httptest"
)

// MakeGitHubAPIServer is used during testing to create an HTTP server to return
// fixtures if the request matches.
func MakeGitHubAPIServer(authToken, wantPath, etag string, response []byte) *httptest.Server {
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != wantPath {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if authToken != "" {
			if auth := r.Header.Get("Authorization"); auth != fmt.Sprintf("token %s", authToken) {
				w.WriteHeader(http.StatusNotFound)
				return
			}
		}
		if auth := r.Header.Get("Authorization"); auth != "" && authToken == "" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if etag == r.Header.Get("If-None-Match") {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		if r.Header.Get("Accept") != "application/vnd.github.chitauri-preview+sha" {
			w.WriteHeader(http.StatusUnsupportedMediaType)
			return
		}
		w.Header().Set("ETag", etag)
		w.Header().Set("Content-Type", "application/json")
		w.Write(response)
	}))
}
