package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/hashicorp/go-multierror"
)

var (
	formTmpl    = template.Must(template.ParseFiles("form.html"))
	mapTmpl     = template.Must(template.ParseFiles("map.html"))
	historyTmpl = template.Must(template.ParseFiles("history.html"))
	dsc         *datastore.Client
)

type IntFieldName string

const (
	id        IntFieldName = "id"
	timestamp IntFieldName = "ts"
	people    IntFieldName = "people"
)

type Snapshot struct {
	People    int8
	Area      string
	TimeStamp time.Time
	Key       *datastore.Key `datastore:"__key__"`
}

func extractNumbers(r *http.Request, fields []IntFieldName) (map[IntFieldName]int, *multierror.Error) {
	var err error
	results := map[IntFieldName]int{}
	var badness *multierror.Error
	for _, n := range fields {
		if results[n], err = strconv.Atoi(r.FormValue(string(n))); err != nil {
			badness = multierror.Append(badness, fmt.Errorf("bad value for \"%s\" field: %v", n, err))
		}
	}
	return results, badness
}

func reportError(statusCode int, msg string, w http.ResponseWriter) {
	log.Print(msg)
	w.WriteHeader(statusCode)
	fmt.Fprintln(w, msg)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	ctx := r.Context()

	area := r.FormValue("area")
	if r.Method == http.MethodGet && area != "" {
		if err := formTmpl.Execute(w, struct{ Area string }{Area: area}); err != nil {
			msg := fmt.Sprintf("Unable to render form template: %v.", err)
			reportError(http.StatusInternalServerError, msg, w)
		}
		return
	}
	floor := r.FormValue("floor")

	if r.Method == http.MethodPost {
		if area == "" {
			reportError(http.StatusBadRequest, "Hidden form field \"area\" not provided.", w)
		}
		fields := []IntFieldName{people}
		values, badness := extractNumbers(r, fields)
		if badness != nil && badness.Len() != 0 {
			msg := fmt.Sprintf("Failure parsing numbers: %v.", badness)
			reportError(http.StatusBadRequest, msg, w)
			return
		}
		record := Snapshot{
			TimeStamp: time.Now(),
			Area:      area,
			People:    int8(values[people]),
		}
		key := datastore.IncompleteKey("Snapshot", nil)
		if _, err := dsc.Put(ctx, key, &record); err != nil {
			msg := fmt.Sprintf("Unable to create new record in DB: %v.", err)
			reportError(http.StatusInternalServerError, msg, w)
			return
		}
		if area[0] == 'U' {
			floor = "2"
		}
	}

	if floor == "" {
		floor = "1"
	}
	if floor != "1" && floor != "2" {
		msg := fmt.Sprintf("Invalid floor: %v.", floor)
		reportError(http.StatusBadRequest, msg, w)
		return
	}
	if err := mapTmpl.Execute(w, struct{ Floor string }{Floor: floor}); err != nil {
		msg := fmt.Sprintf("Unable to render map template: %v.", err)
		reportError(http.StatusInternalServerError, msg, w)
	}
}

type KeyedRecord struct {
	Snapshot
	Key *datastore.Key
}

type Listing struct {
	Area    string
	Records []KeyedRecord
}

func addKeysToSnapshots(snapshots []Snapshot, keys []*datastore.Key) []KeyedRecord {
	records := []KeyedRecord{}
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		log.Printf("Unable to load location from TZ DB.")
	}
	for i, r := range snapshots {
		if ny != nil {
			r.TimeStamp = r.TimeStamp.In(ny)
		}
		records = append(records, KeyedRecord{Snapshot: r, Key: keys[i]})
	}
	return records
}

func historyHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	area := r.FormValue("area")
	if area == "" {
		reportError(http.StatusBadRequest, "Required parameter \"area\" not provided.", w)
		return
	}
	switch r.Method {
	case http.MethodGet:
		snapshots := []Snapshot{}
		keys, err := dsc.GetAll(ctx, datastore.NewQuery("Snapshot").Filter("Area =", area).Order("-TimeStamp"), &snapshots)
		if err != nil {
			reportError(http.StatusInternalServerError, err.Error(), w)
			return
		}
		records := addKeysToSnapshots(snapshots, keys)
		if err = historyTmpl.Execute(w, Listing{Area: area, Records: records}); err != nil {
			msg := fmt.Sprintf("Unable to render history template: %v.", err)
			reportError(http.StatusInternalServerError, msg, w)
		}
	case http.MethodPost:
		fields := []IntFieldName{id, timestamp, people}
		values, badness := extractNumbers(r, fields)
		if badness != nil && badness.Len() != 0 {
			msg := fmt.Sprintf("Failure parsing integer parameter: %v.", badness)
			reportError(http.StatusBadRequest, msg, w)
			return
		}
		record := Snapshot{
			TimeStamp: time.Unix(int64(values[timestamp]), 0),
			Area:      area,
			People:    int8(values[people]),
		}
		key := datastore.IDKey("Snapshot", int64(values[id]), nil)
		if _, err := dsc.Put(ctx, key, &record); err != nil {
			msg := fmt.Sprintf("Unable to update record in DB: %v.", err)
			reportError(http.StatusInternalServerError, msg, w)
			return
		}
		w.Header()["Location"] = append(w.Header()["Location"], fmt.Sprintf("/history?area=%s", area))
		http.Redirect(w, r, "/", http.StatusSeeOther)
	default:
		reportError(http.StatusMethodNotAllowed, fmt.Sprintf("Unsupported HTTP method %s.", r.Method), w)
	}
}

func csvHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	snapshots := []Snapshot{}
	keys, err := dsc.GetAll(ctx, datastore.NewQuery("Snapshot").Order("-TimeStamp"), &snapshots)
	if err != nil {
		reportError(http.StatusInternalServerError, err.Error(), w)
		return
	}
	records := addKeysToSnapshots(snapshots, keys)
	allRows := [][]string{[]string{"DateTime", "Area", "People"}}
	for _, r := range records {
		allRows = append(allRows, []string{r.TimeStamp.Format("2006-01-02 @ 3:04 pm"), r.Area, strconv.Itoa(int(r.People))})
	}
	w.Header().Set("Content-type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=census-data.csv")
	if err := csv.NewWriter(w).WriteAll(allRows); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	ctx := context.Background()
	var err error
	dsc, err = datastore.NewClient(ctx, "census-199900")
	if err != nil {
		log.Fatalf("Cannot establish connection to DataStore: %s", err)
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}
	http.Handle("/static/", http.FileServer(http.Dir("static")))
	http.HandleFunc("/history", historyHandler)
	http.HandleFunc("/csv", csvHandler)
	http.HandleFunc("/", indexHandler)
	log.Printf("Listening on port %s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}
