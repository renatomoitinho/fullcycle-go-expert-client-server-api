package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const (
	quotationSaveTimeout = 10 * time.Millisecond
	quotationLoadTimeout = 200 * time.Millisecond
)

type quotation struct {
	ID         int     `json:"id" gorm:"primaryKey"`
	Code       string  `json:"code"`
	Codein     string  `json:"codein"`
	Name       string  `json:"name"`
	High       float64 `json:"high,string"`
	Low        float64 `json:"low,string"`
	VarBid     float64 `json:"varBid,string"`
	PctChange  float64 `json:"pctChange,string"`
	Bid        float64 `json:"bid,string"`
	Ask        float64 `json:"ask,string"`
	Timestamp  int64   `json:"timestamp,string"`
	CreateDate string  `json:"create_date"`
}

type quotationServer struct {
	db          *gorm.DB
	saveTimeout time.Duration
	loadTimeout time.Duration
}

func newQuotationServer(db *gorm.DB) *quotationServer {
	return &quotationServer{
		db:          db,
		saveTimeout: quotationSaveTimeout,
		loadTimeout: quotationLoadTimeout,
	}
}

func (q *quotationServer) save(ctx context.Context, quotation *quotation) (err error) {
	timeout, cancel := context.WithTimeout(ctx, q.saveTimeout)
	defer cancel()
	err = q.db.WithContext(timeout).Create(quotation).Error
	if err != nil {
		log.Println("error in save quotation: ", err.Error())
		return err
	}
	return nil
}

func (q *quotationServer) loadFromApi(ctx context.Context) (*quotation, error) {
	timeout, cancel := context.WithTimeout(ctx, q.loadTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(timeout, http.MethodGet, "https://economia.awesomeapi.com.br/json/last/USD-BRL", nil)
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
	var values map[string]quotation
	err = json.NewDecoder(resp.Body).Decode(&values)
	if err != nil {
		return nil, err
	}
	if val, ok := values["USDBRL"]; ok {
		return &val, nil
	}
	return nil, errors.New("not found USDBRL")
}

func (q *quotationServer) writeError(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(err.Error()))
}

func (q *quotationServer) writeOK(w http.ResponseWriter, model any) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(model)
	if err != nil {
		q.writeError(w, err)
		return
	}
}

func (q *quotationServer) httpHandler(w http.ResponseWriter, r *http.Request) {
	model, err := q.loadFromApi(r.Context())
	if err != nil {
		q.writeError(w, err)
		return
	}
	err = q.save(r.Context(), model)
	if err != nil {
		q.writeError(w, err)
		return
	}
	q.writeOK(w, model)
}

func (q *quotationServer) ListenAndServe() {
	mux := http.NewServeMux()
	mux.HandleFunc("/cotacao", q.httpHandler)
	server := http.Server{Addr: ":8080", Handler: mux}
	defer func() {
		_ = server.Close()
	}()
	log.Fatal(server.ListenAndServe())
}

func closeDb(db *gorm.DB) {
	dbSql, err := db.DB()
	if err != nil {
		log.Fatal(err.Error())
	}
	err = dbSql.Close()
	if err != nil {
		log.Fatal(err.Error())
	}
}

func initDb() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("quotation.db"), &gorm.Config{})
	if err != nil {
		log.Fatal(err.Error())
	}
	err = db.AutoMigrate(&quotation{})
	if err != nil {
		log.Fatal(err.Error())
	}
	return db
}

func main() {
	db := initDb()
	defer closeDb(db)
	quotation := newQuotationServer(db)
	quotation.ListenAndServe()
}
