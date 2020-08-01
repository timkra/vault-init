// Copyright 2018 Google Inc. All Rights Reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
)

var (
	rootTokenSecretID    string
	recoveryKeysSecretID string
	vaultAddr            string
	httpClient           http.Client

	vaultStoredShares      int
	vaultRecoveryShares    int
	vaultRecoveryThreshold int
)

// InitRequest holds a Vault init request.
type InitRequest struct {
	StoredShares      int `json:"stored_shares"`
	RecoveryShares    int `json:"recovery_shares"`
	RecoveryThreshold int `json:"recovery_threshold"`
}

// InitResponse holds a Vault init response.
type InitResponse struct {
	Keys       []string `json:"recovery_keys"`
	KeysBase64 []string `json:"recovery_keys_base64"`
	RootToken  string   `json:"root_token"`
}

// RootTokenSecretKV Holds the Key-Value pair for Secrets Manager
type RootTokenSecretKV struct {
	Name string `json:"root-token"`
}

func main() {
	log.Println("Starting the vault-init service...")

	vaultAddr = os.Getenv("VAULT_ADDR")
	if vaultAddr == "" {
		vaultAddr = "https://127.0.0.1:8200"
	}

	rootTokenSecretID = os.Getenv("ROOT_TOKEN_SECRET_ID")
	if rootTokenSecretID == "" {
		log.Fatal("ROOT_TOKEN_SECRET_ID must be set and not empty")
	}

	recoveryKeysSecretID = os.Getenv("RECOVERY_KEYS_SECRET_ID")
	if recoveryKeysSecretID == "" {
		log.Fatal("RECOVERY_KEYS_SECRET_ID must be set and not empty")
	}

	vaultStoredShares = intFromEnv("VAULT_STORED_SHARES", 1)
	vaultRecoveryShares = intFromEnv("VAULT_RECOVERY_SHARES", 1)
	vaultRecoveryThreshold = intFromEnv("VAULT_RECOVERY_THRESHOLD", 1)

	checkInterval := durFromEnv("CHECK_INTERVAL", 10*time.Second)

	httpClient = http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	signalCh := make(chan os.Signal)
	signal.Notify(signalCh,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGKILL,
	)

	stop := func() {
		log.Printf("Shutting down")
		os.Exit(0)
	}

	for {
		select {
		case <-signalCh:
			stop()
		default:
		}
		response, err := httpClient.Head(vaultAddr + "/v1/sys/health")

		if response != nil && response.Body != nil {
			response.Body.Close()
		}

		if err != nil {
			log.Println(err)
			time.Sleep(checkInterval)
			continue
		}

		switch response.StatusCode {
		case 200:
			log.Println("Vault is initialized and unsealed.")
		case 429:
			log.Println("Vault is unsealed and in standby mode.")
		case 501:
			log.Println("Vault is not initialized.")
			log.Println("Initializing...")
			initialize()
		default:
			log.Printf("Vault is in an unknown state. Status code: %d", response.StatusCode)
		}

		log.Printf("Next check in %s", checkInterval)

		select {
		case <-signalCh:
			stop()
		case <-time.After(checkInterval):
		}
	}
}

func initialize() {
	initRequest := InitRequest{
		StoredShares:      vaultStoredShares,
		RecoveryShares:    vaultRecoveryShares,
		RecoveryThreshold: vaultRecoveryThreshold,
	}

	initRequestData, err := json.Marshal(&initRequest)
	if err != nil {
		log.Println(err)
		return
	}

	r := bytes.NewReader(initRequestData)
	request, err := http.NewRequest("PUT", vaultAddr+"/v1/sys/init", r)
	if err != nil {
		log.Println(err)
		return
	}

	response, err := httpClient.Do(request)
	if err != nil {
		log.Println(err)
		return
	}
	defer response.Body.Close()

	initRequestResponseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Println(err)
		return
	}

	if response.StatusCode != 200 {
		log.Printf("init: non 200 status code: %d", response.StatusCode)
		return
	}

	var initResponse InitResponse

	if err := json.Unmarshal(initRequestResponseBody, &initResponse); err != nil {
		log.Println(err)
		return
	}

	RootTokenSecretKV := RootTokenSecretKV{
		Name: initResponse.RootToken,
	}

	RootTokenSecretString, err := json.Marshal(RootTokenSecretKV)
	if err != nil {
		log.Fatal(err)
		return
	}

	log.Println("Storing root token and recovery keys in Secrets Manager...")
	sess := session.Must(session.NewSession())

	svc := secretsmanager.New(sess)

	rootTokenInput := &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(rootTokenSecretID),
		SecretString: aws.String(string(RootTokenSecretString)),
	}

	_, err = svc.PutSecretValue(rootTokenInput)
	if err != nil {
		log.Fatal(err)
		return
	}

	var recoveryKeys = make(map[string]string)

	for index, element := range initResponse.Keys {
		recoveryKeys["recovery-key-"+strconv.Itoa(index+1)] = element
	}

	jsonRecoveryKeys, _ := json.Marshal(recoveryKeys)

	recoveryKeysInput := &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(recoveryKeysSecretID),
		SecretString: aws.String(string(jsonRecoveryKeys)),
	}

	_, err = svc.PutSecretValue(recoveryKeysInput)
	if err != nil {
		log.Fatal(err)
		return
	}

	log.Println("Initialization complete.")
}

func boolFromEnv(env string, def bool) bool {
	val := os.Getenv(env)
	if val == "" {
		return def
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		log.Fatalf("failed to parse %q: %s", env, err)
	}
	return b
}

func intFromEnv(env string, def int) int {
	val := os.Getenv(env)
	if val == "" {
		return def
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		log.Fatalf("failed to parse %q: %s", env, err)
	}
	return i
}

func durFromEnv(env string, def time.Duration) time.Duration {
	val := os.Getenv(env)
	if val == "" {
		return def
	}
	r := val[len(val)-1]
	if r >= '0' || r <= '9' {
		val = val + "s" // assume seconds
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		log.Fatalf("failed to parse %q: %s", env, err)
	}
	return d
}
