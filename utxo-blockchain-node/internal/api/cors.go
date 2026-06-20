package api

import "net/http"

// corsMiddleware wraps next with permissive CORS headers suitable for
// local development frontends (e.g. Vite on http://localhost:5173).
//
// Preflight OPTIONS requests are short-circuited with 204. All other
// requests proceed to next with the appropriate Access-Control-* headers
// attached to the response. This is intentionally permissive — do not
// enable in a production deployment without restricting the allowed
// origin via Options.EnableCORS=false.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		h.Set("Access-Control-Max-Age", "600")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
