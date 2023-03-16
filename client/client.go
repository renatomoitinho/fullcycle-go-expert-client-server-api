package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"text/template"
	"time"
)

const (
	quotationLoadTimeout = 300 * time.Millisecond
)

type quotation struct {
	Bid float64 `json:"bid,string"`
}

type quotationClient struct {
	template    *template.Template
	readTimeout time.Duration
}

func newQuotationClient() *quotationClient {
	return &quotationClient{
		template:    template.Must(template.New("test").Parse("Dólar: {{.Bid}}\n")),
		readTimeout: quotationLoadTimeout,
	}
}

func (cli *quotationClient) loadFromApi(ctx context.Context, url string) (*quotation, error) {
	timeout, cancel := context.WithTimeout(ctx, cli.readTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(timeout, http.MethodGet, url, nil)
	if err != nil {
		log.Println("error in loadFromApi: ", err.Error())
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("error in loadFromApi: ", err.Error())
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, errors.New("error in loadFromApi: " + string(b))
	}

	var q quotation
	err = json.NewDecoder(resp.Body).Decode(&q)
	if err != nil {
		return nil, err
	}
	return &q, nil
}

func (cli *quotationClient) openWriter() (*os.File, error) {
	f, err := os.OpenFile("cotacao.txt", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	return f, err
}

func (cli *quotationClient) Execute(ctx context.Context) error {
	f, err := cli.openWriter()
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	q, err := cli.loadFromApi(ctx, "http://localhost:8080/cotacao")
	if err != nil {
		return err
	}
	return cli.template.Execute(f, q)
}

func main() {
	cli := newQuotationClient()
	err := cli.Execute(context.Background())
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("well done!")
}
