package main

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"os/exec"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/joho/godotenv"
)

type Response struct {
	Result string
	Err    error
}

// checkNet pings 8.8.8.8 to check if there is an internet connection
func checkNet(r chan Response) {

	cmd := exec.Command("ping", "8.8.8.8")

	var out bytes.Buffer

	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		r <- Response{"", errors.New("no internet")}
		os.Exit(1)
	}

	r <- Response{"Internet", nil}
}

// countDays counts the number of days from day one to today
func countDays() int {
	date := time.Now()
	fmt.Println(date)

	format := "2006-01-02 15:04:05"
	then, _ := time.Parse(format, "2022-11-13 08:58:06")
	fmt.Println(then)

	diff := date.Sub(then)

	//func Since(t Time) Duration
	//Since returns the time elapsed since t.
	//It is shorthand for time.Now().Sub(t).

	return int(diff.Hours() / 24) // number of days
}

// loadDevotional scraps daily devotional text and packages the message as an email
func loadDevotionalEmail() ([]byte, error) {
	dev, err := http.Get(os.Getenv("WEB"))
	if err != nil {
		return []byte("couldnt scrap web page"), errors.New("couldnt scrap webpage")
	}

	defer dev.Body.Close()

	if dev.StatusCode != 200 {
		return []byte("webpage returned non 200 code"), errors.New("non 200 status code")
	}

	doc, err := goquery.NewDocumentFromReader(dev.Body)
	if err != nil {
		log.Fatal(err)
	}

	var devotional []string
	daysFromStart := countDays()
	subject := fmt.Sprintf("Subject: Day %d of devotional\r\n", daysFromStart)
	// Find the review items
	doc.Find("p").Each(func(i int, s *goquery.Selection) {
		// For each item found, get the title
		devotional = append(devotional, "To:brian_tum@outlook.com\r\n"+subject)
		devotional = append(devotional, s.Text())
	})

	buf := &bytes.Buffer{}
	gob.NewEncoder(buf).Encode(devotional)
	ds := buf.Bytes()
	return []byte(ds), nil
}

// sendEmail sends devotional emails to me if there is an internet connection, daily
func sendEmail(body []byte) {
	godotenv.Load()
	from := os.Getenv("MAIL")
	password := os.Getenv("PWD")

	to := []string{"brian_tum@outlook.com"}
	host := "smtp.gmail.com"
	port := "587"
	auth := smtp.PlainAuth("", from, password, host)

	if err := smtp.SendMail(host+":"+port, auth, from, to, body); err != nil {
		log.Println(err)
		os.Exit(1)
	}

}

func main() {
	ticker := time.NewTicker(1 * time.Minute)
	done := make(chan bool)

	body, _ := loadDevotionalEmail()
	response := make(chan Response)

	go checkNet(response)
	net_ok := <-response

	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if net_ok.Err == nil {
					sendEmail(body)
					done <- true
					ticker.Stop()
				}
			}
		}
	}()
}
