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
		Wallet int64 `json:"wallet_id"`
		Amount int64 `json:"amount"`
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
	err = s.repo.replenishWallet(payload.ID, query.Amount, query.Wallet)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "Error while topping up the wallet")
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Replenished the wallet")
}

func (s *Server) transfer(w http.ResponseWriter, r *http.Request) {
	query := struct {
		ToWallet   int64 `json:"to_wallet_id"`
		FromWallet int64 `json:"from_wallet_id"`
		Amount     int64 `json:"amount"`
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
	err = s.repo.transferMoney(payload.ID, query.FromWallet, query.ToWallet, query.Amount)
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

	http.ListenAndServe(fmt.Sprintf(":%s", config.Port), r)
}
