package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const (
	CODE_LENGTH = 12
)

var invalidWallet error = errors.New("invalid wallet")
var insufficientBalance error = errors.New("invalid balance")

type Repo struct {
	db *sql.DB
}

type Wallet struct {
	ID        int64     `json:"id"`
	Owner     int64     `json:"owner"`
	Code      string    `json:"code"`
	CreatedAt time.Time `json:"created_at"`
	Balance   int64     `json:"balance"`
}

func NewRepo(DSN string) (*Repo, error) {
	db, err := sql.Open("mysql", DSN)
	if err != nil {
		return nil, err
	}
	if err = db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return &Repo{
		db: db,
	}, nil
}

func generateCode() string {
	id := make([]byte, CODE_LENGTH)
	for i := 0; i < CODE_LENGTH; i++ {
		id[i] = byte('0' + rand.Intn(10))
	}
	return string(id)
}

func (r *Repo) createWallet(owner int64) (*Wallet, error) {
	var query string
	var err error

	code := generateCode()
	query = "INSERT INTO wallets (owner, code) VALUES (?, ?)"
	wallet := &Wallet{
		Owner: owner,
		Code:  code,
	}

	_, err = r.db.Exec(query, owner, code)
	if err != nil {
		return nil, fmt.Errorf("createWallet: %w", err)
	}

	err = r.db.QueryRow("SELECT id, created_at, balance FROM wallets WHERE code = ?", code).Scan(&wallet.ID, &wallet.CreatedAt, &wallet.Balance)
	if err != nil {
		return nil, fmt.Errorf("createWallet: %w", err)
	}

	return wallet, nil
}

func (r *Repo) getWallets(owner *int64) ([]*Wallet, error) {
	var query string
	if owner == nil {
		query = "SELECT id, owner, code, created_at, balance FROM wallets"
	} else {
		query = "SELECT id, owner, code, created_at, balance FROM wallets WHERE owner = ?"
	}

	var rows *sql.Rows
	var err error

	if owner == nil {
		rows, err = r.db.Query(query)
	} else {
		rows, err = r.db.Query(query, *owner)
	}
	if err != nil {
		return nil, fmt.Errorf("getWallets: %w", err)
	}
	defer rows.Close()

	wallets := make([]*Wallet, 0)
	for rows.Next() {
		wallet := &Wallet{}
		err = rows.Scan(&wallet.ID, &wallet.Owner, &wallet.Code, &wallet.CreatedAt, &wallet.Balance)
		if err != nil {
			return nil, fmt.Errorf("getWallets: %w", err)
		}
		wallets = append(wallets, wallet)
	}

	return wallets, nil
}

func (r *Repo) replenishWallet(owner int64, amount int64, walletID int64) error {
	query := "SELECT COUNT(*) FROM wallets WHERE owner = ? AND id = ?"
	var count int64
	err := r.db.QueryRow(query, owner, walletID).Scan(&count)
	if err != nil {
		return fmt.Errorf("replenishWallet: %w", err)
	}
	if count != 1 {
		return fmt.Errorf("replenishWallet: %w", invalidWallet)
	}

	query = "UPDATE wallets SET balance = balance + ? WHERE owner = ? AND id = ?"
	_, err = r.db.Exec(query, amount, owner, walletID)
	if err != nil {
		return fmt.Errorf("replenishWallet: %w", err)
	}

	return nil
}

func (r *Repo) transferMoney(owner int64, fromWalletID int64, toWalletID int64, amount int64) error {
	query := "SELECT balance FROM wallets WHERE owner = ? AND id = ?"
	var balance int64
	err := r.db.QueryRow(query, owner, fromWalletID).Scan(&balance)

	if err != nil {
		return fmt.Errorf("transferMoney: %w", err)
	}

	if balance < amount {
		return fmt.Errorf("transferMoney: %w", insufficientBalance)
	}

	ctx := context.Background()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "UPDATE wallets SET balance = balance - ? WHERE id = ?", amount, fromWalletID)
	if err != nil {
		tx.Rollback()
		return err
	}
	_, err = tx.ExecContext(ctx, "UPDATE wallets SET balance = balance + ? WHERE id = ?", amount, toWalletID)
	if err != nil {
		tx.Rollback()
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO transfers (amount, from_wallet_id, to_wallet_id) VALUES (?, ?, ?)", amount, fromWalletID, toWalletID)
	if err != nil {
		tx.Rollback()
		return err
	}
	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}
