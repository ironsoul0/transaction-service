package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
)

const (
	ADMIN_ROLE = "admin"
)

type TokenPayload struct {
	ID       int64  `json:"id"`
	IIN      string `json:"iin"`
	Username string `json:"username"`
	Role     string `json:"payload"`
}

func (s *Server) parseToken(token string, isAccess bool) (*TokenPayload, error) {
	JWTToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Failed to extract token metadata, unexpected signing method: %v", token.Header["alg"])
		}
		if isAccess {
			return []byte(s.accessSecret), nil
		}
		return []byte(s.refreshSecret), nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := JWTToken.Claims.(jwt.MapClaims)

	if ok && JWTToken.Valid {
		payload, ok := claims["payload"].(string)
		if !ok {
			return nil, fmt.Errorf("Field payload not found")
		}

		exp, ok := claims["exp"].(float64)
		if !ok {
			return nil, fmt.Errorf("Field exp not found")
		}

		expiredTime := time.Unix(int64(exp), 0)
		log.Printf("Expired: %v", expiredTime)
		if time.Now().After(expiredTime) {
			return nil, fmt.Errorf("Token expired")
		}
		tokenPayload := TokenPayload{}
		json.Unmarshal([]byte(payload), &tokenPayload)
		fmt.Println("payload token", tokenPayload)
		return &tokenPayload, nil
	}

	return nil, fmt.Errorf("Invalid token")
}

func extractToken(r *http.Request) (token string, err error) {
	header := string(r.Header.Get("Authorization"))
	if header == "" {
		err = fmt.Errorf("Authorization header not found")
		return
	}
	parsedHeader := strings.Split(header, " ")
	if len(parsedHeader) != 2 || parsedHeader[0] != "Bearer" {
		err = fmt.Errorf("Invalid authorization header")
		return
	}

	token = parsedHeader[1]
	return
}

func getPayload(r *http.Request) (*TokenPayload, error) {
	payload, ok := r.Context().Value("payload").(*TokenPayload)
	if !ok {
		return nil, fmt.Errorf("getPayload: unable to get payload from request")
	}
	return payload, nil
}
