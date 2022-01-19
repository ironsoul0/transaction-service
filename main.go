package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"transactions-service/util"

	"github.com/gorilla/mux"
)

type Server struct {
	repo *Repo

	accessSecret  string
	refreshSecret string
}

func (s *Server) authorizatonCheckMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := extractToken(r)
		if err != nil {
			log.Printf("Extract access token error: %v", err)
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, "Unauthorized to perform this action")
			return
		}

		tokenPayload, err := s.parseToken(token, true)
		if err != nil {
			log.Printf("Parse access token error: %v", err)
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, "Unauthorized to perform this action")
			return
		}

		ctx := context.WithValue(r.Context(), "payload", tokenPayload)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) adminCheckMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload := r.Context().Value("payload").(*TokenPayload)

		if payload == nil || payload.Role != ADMIN_ROLE {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, "Admin access required")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) createWallet(w http.ResponseWriter, r *http.Request) {
	payload, err := getPayload(r)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	wallet, err := s.repo.createWallet(payload.ID)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "Error while creating the wallet")
		fmt.Println(err)
		return
	}

	w.WriteHeader(http.StatusOK)
	buf, _ := json.MarshalIndent(wallet, "", " ")
	fmt.Fprint(w, string(buf))
}

func (s *Server) replenishWallet(w http.ResponseWriter, r *http.Request) {
	query := struct {
		WalletCode string `json:"wallet_code"`
		Amount     int64  `json:"amount"`
	}{}
	err := json.NewDecoder(r.Body).Decode(&query)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Invalid query issued")
		return
	}

	payload, err := getPayload(r)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	err = s.repo.replenishWallet(payload.ID, query.Amount, query.WalletCode)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Replenished the wallet")
}

func (s *Server) transfer(w http.ResponseWriter, r *http.Request) {
	query := struct {
		ToWalletCode   string `json:"to_wallet_code"`
		FromWalletCode string `json:"from_wallet_code"`
		Amount         int64  `json:"amount"`
	}{}
	err := json.NewDecoder(r.Body).Decode(&query)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Invalid query issued")
		return
	}
	payload, err := getPayload(r)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	err = s.repo.transferMoney(payload.ID, query.FromWalletCode, query.ToWalletCode, query.Amount)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Transferred the amount")
}

func (s *Server) getWallets(w http.ResponseWriter, r *http.Request) {
	payload, err := getPayload(r)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	wallets, err := s.repo.getWallets(&payload.ID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "Error while getting the wallets")
		fmt.Println(err)
		return
	}

	buf, _ := json.MarshalIndent(wallets, "", " ")
	fmt.Fprint(w, string(buf))
}

func (s *Server) listWallets(w http.ResponseWriter, r *http.Request) {
	wallets, err := s.repo.getWallets(nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "Error while getting the wallets")
		fmt.Println(err)
		return
	}
	buf, _ := json.MarshalIndent(wallets, "", " ")
	fmt.Fprint(w, string(buf))
}

func CORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		w.Header().Set("Access-Control-Allow-Origin", origin)
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token, Authorization")
			return
		} else {
			h.ServeHTTP(w, r)
		}
	})
}

func main() {
	config, err := util.LoadConfig(".")
	if err != nil {
		log.Fatalf("Loading config error: %v", err)
	}

	repo, err := NewRepo(config.DBSource)
	if err != nil {
		log.Fatalf("MySQL error: %v", err)
	}

	server := &Server{
		repo:          repo,
		accessSecret:  config.AccessSecret,
		refreshSecret: config.RefreshSecret,
	}

	r := mux.NewRouter()

	appRouter := r.PathPrefix("/").Subrouter()
	appRouter.Use(server.authorizatonCheckMiddleware)
	appRouter.HandleFunc("/wallet", server.createWallet).Methods("POST")
	appRouter.HandleFunc("/wallets", server.getWallets).Methods("GET")
	appRouter.HandleFunc("/replenish", server.replenishWallet).Methods("POST")
	appRouter.HandleFunc("/transfer", server.transfer).Methods("POST")

	adminRouter := r.PathPrefix("/").Subrouter()
	adminRouter.Use(server.authorizatonCheckMiddleware, server.adminCheckMiddleware)
	adminRouter.HandleFunc("/list_wallets", server.listWallets).Methods("GET")

	http.ListenAndServe(fmt.Sprintf(":%s", config.Port), CORS(r))
}
