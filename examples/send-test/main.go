// Command send-test is a minimal runnable example demonstrating how to
// send an SMS and handle errors with the unisms client. It mirrors the
// PHP reference implementation's examples/send-test.php.
//
// Run it with:
//
//	UNISMS_SECRET_KEY=sk_your_secret_key go run ./examples/send-test
//
// or edit the secretKey constant below directly.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/taliffsss/unisms-go"
)

func main() {
	secretKey := os.Getenv("UNISMS_SECRET_KEY")
	if secretKey == "" {
		secretKey = "sk_15eba0c5-b301-4626-ae63-6a47c1b2d180"
	}

	recipient := "09055251658"
	message := "test"

	client, err := unisms.New(
		secretKey,
		unisms.WithTimeout(30*time.Second),
		unisms.WithMaxRetries(2),
	)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Send(ctx, unisms.SendRequest{
		Recipient: recipient,
		Content:   message,
	})
	if err != nil {
		var apiErr *unisms.APIError
		var transportErr *unisms.TransportError
		var validationErr *unisms.ValidationError

		switch {
		case errors.As(err, &apiErr):
			fmt.Printf("API error (%d): %s\n", apiErr.StatusCode, apiErr.ResponseBody)
		case errors.As(err, &transportErr):
			fmt.Println("Transport error:", transportErr.Error())
		case errors.As(err, &validationErr):
			fmt.Println("Validation error:", validationErr.Error())
		default:
			fmt.Println("Error:", err)
		}
		os.Exit(1)
	}

	fmt.Println("Sent successfully:")
	fmt.Printf("%#v\n", resp)

	// Example of checking message status once a reference id is known.
	if refID := resp.String("reference_id"); refID != "" {
		status, err := client.GetMessage(ctx, refID)
		if err != nil {
			fmt.Println("Error fetching status:", err)
			return
		}
		fmt.Println("Status:", status)
	}
}
