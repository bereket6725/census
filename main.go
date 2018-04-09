package main

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
)

var (
	formTmpl = template.Must(template.ParseFiles("form.html"))
	mapTmpl  = template.Must(template.ParseFiles("map.html"))
)

type People struct {
	Total, Grouped, Solitary, Asleep int
}

type Snapshot struct {
	People
	Decibels, Laptops int
	Area              string
	TimeStamp         time.Time
}

func extractNumbers(r *http.Request, fields []string) (map[string]int, appengine.MultiError) {
	var err error
	results := map[string]int{}
	var badness appengine.MultiError
	for _, n := range fields {
		if results[n], err = strconv.Atoi(r.FormValue(n)); err != nil {
			badness = append(badness, fmt.Errorf("bad value for \"%s\" field: %v", n, err))
		}
	}
	return results, badness
}

func reportError(ctx context.Context, statusCode int, msg string, w http.ResponseWriter) {
	log.Errorf(ctx, msg)
	w.WriteHeader(statusCode)
	fmt.Fprintln(w, msg)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	area := r.FormValue("area")
	if r.Method == http.MethodGet && len(area) > 0 {
		formTmpl.Execute(w, struct{ Area string }{Area: area})
		return
	}

	if r.Method == http.MethodPost {
		ctx := appengine.NewContext(r)
		if len(area) == 0 {
			reportError(ctx, http.StatusBadRequest, "Hidden form field \"area\" not provided.", w)
		}
		fields := []string{"total", "grouped", "solitary", "asleep", "laptops"}
		if len(r.FormValue("decibels")) > 0 {
			fields = append(fields, "decibels")
		}
		values, badness := extractNumbers(r, fields)
		if len(badness) != 0 {
			msg := fmt.Sprintf("Failure parsing numbers: %v.", badness)
			reportError(ctx, http.StatusBadRequest, msg, w)
			return
		}
		record := Snapshot{
			TimeStamp: time.Now(),
			Area:      area,
			People: People{
				Total:    values["total"],
				Grouped:  values["grouped"],
				Solitary: values["solitary"],
				Asleep:   values["asleep"],
			},
			Laptops:  values["laptops"],
			Decibels: values["decibels"],
		}
		if record.Total != record.Grouped+record.Solitary+record.Asleep {
			msg := fmt.Sprintf("Total (%d) != grouped (%d) + solitary (%d) + asleep (%d).",
				record.Total, record.Grouped, record.Solitary, record.Asleep)
			reportError(ctx, http.StatusBadRequest, msg, w)
			return
		}
		key := datastore.NewIncompleteKey(ctx, "Snapshot", nil)
		if _, err := datastore.Put(ctx, key, &record); err != nil {
			msg := fmt.Sprintf("Unable to create new record in DB: %v.", err)
			reportError(ctx, http.StatusInternalServerError, msg, w)
			return
		}
	}

	mapTmpl.Execute(w, nil)
}

func main() {
	http.HandleFunc("/", indexHandler)
	appengine.Main()
}
