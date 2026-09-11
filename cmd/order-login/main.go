package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	"example.com/verified-order-login/internal/infrai"
	"example.com/verified-order-login/internal/orders"
)

type loginRequest struct {
	OrderID string `json:"order_id"`
	Phone   string `json:"phone"`
	Code    string `json:"code,omitempty"`
}

func main() {
	sms, err := infrai.NewSMSClient(os.Getenv("INFRAI_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	access := orders.NewAccess(sms)
	mux := http.NewServeMux()

	mux.HandleFunc("POST /login/code", func(w http.ResponseWriter, r *http.Request) {
		var input loginRequest
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := access.SendCode(r.Context(), input.OrderID, input.Phone); err != nil {
			writeError(w, http.StatusUnauthorized, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "code_sent", "order_id": input.OrderID})
	})

	mux.HandleFunc("POST /login/verify", func(w http.ResponseWriter, r *http.Request) {
		var input loginRequest
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		order, err := access.VerifyAndRelease(r.Context(), input.OrderID, input.Phone, input.Code)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "order_released", "order": order})
	})

	log.Println("order login listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
