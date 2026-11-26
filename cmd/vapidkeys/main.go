// Command vapidkeys prints a fresh VAPID key pair for Web Push.
//
// The output is shell-sourceable, so a local run can pipe it into an env file:
//
//	go run ./cmd/vapidkeys >> .env
//
// The private key is a secret. In production it belongs in Secret Manager, never
// in the repository.
package main

import (
	"fmt"
	"io"
	"os"

	webpush "github.com/SherClockHolmes/webpush-go"
)

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "vapidkeys: %v\n", err)
		os.Exit(1)
	}
}

// run writes the pair as VAPID_PUBLIC_KEY and VAPID_PRIVATE_KEY assignments.
func run(w io.Writer) error {
	privateKey, publicKey, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		return fmt.Errorf("generate VAPID keys: %w", err)
	}
	if _, err := fmt.Fprintf(w, "VAPID_PUBLIC_KEY=%s\nVAPID_PRIVATE_KEY=%s\n", publicKey, privateKey); err != nil {
		return fmt.Errorf("write VAPID keys: %w", err)
	}
	return nil
}
