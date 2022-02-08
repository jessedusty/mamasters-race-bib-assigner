package main

import (
	"encoding/csv"
	"fmt"
	"github.com/gocarina/gocsv"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

type LoanerBib struct {
	Slot string `csv:"Slot"`
	Bib  string `csv:"Bib"`
}

type RaceEntry struct {
	NEMSBibNumber    string
	MIDBibNumber     string
	USSA             string
	FIS              string
	FirstName        string
	LastName         string
	YOB              string
	Gender           string
	Team             string
	RegistrationDate string
	USSAMembership   string
	NASTAR           string
	SeasonPass       string
	ExtraCol         string
}

func (e *RaceEntry) HomeBib() string {
	return cleanBib(e.MIDBibNumber)
}

func (e *RaceEntry) AwayBib() string {
	return cleanBib(e.NEMSBibNumber)
}

func (e *RaceEntry) SetBib(bib string) {
	e.MIDBibNumber = cleanBib(bib)
}

func (e *RaceEntry) PersonKey() string {
	return strings.Trim(e.USSA+e.FirstName+e.LastName+e.YOB, " ")
}

func (e *RaceEntry) LogName() string {
	return fmt.Sprintf("%s %s", e.FirstName, e.LastName)
}

func cleanBib(bib string) string {
	return strings.TrimSpace(bib)
}

func (s *BibSolver) loadLoanerBibs() error {
	loanerBibFile, err := os.Open("Available Loaner bibs - masters updated 1-30-22.csv")
	if err != nil {
		return err
	}

	var loanerBibs []LoanerBib

	err = gocsv.UnmarshalFile(loanerBibFile, &loanerBibs)
	if err != nil {
		return err
	}

	var retVal []string
	for _, bib := range loanerBibs {
		retVal = append(retVal, cleanBib(bib.Bib))
	}

	s.loanerBibs = retVal
	return nil
}

func rowToRaceEntry(row []string) (RaceEntry, error) {

	var e RaceEntry

	numStructFields := reflect.ValueOf(e).NumField()
	if len(row) != numStructFields {
		return RaceEntry{}, fmt.Errorf("number of field mis-match, %d != %d", len(row), numStructFields)
	}

	raceEntryReflection := reflect.ValueOf(&e).Elem()

	for i := range row {
		f := raceEntryReflection.Field(i)
		if !f.CanSet() {
			return RaceEntry{}, fmt.Errorf("could not set field")
		} else {
			f.SetString(row[i])
		}
	}
	return e, nil
}

func RaceEntryToRow(e RaceEntry) []string {
	numStructFields := reflect.ValueOf(e).NumField()
	raceEntryReflection := reflect.ValueOf(&e).Elem()

	cells := make([]string, numStructFields)
	for i := 0; i < numStructFields; i++ {
		f := raceEntryReflection.Field(i)
		cells[i] = f.String()
	}

	return cells
}

const skipLines = 2

func (s *BibSolver) loadRaceFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}

	csvReader := csv.NewReader(f)

	day := RaceDay{
		Path: path,
	}

	// Skip lines
	for i := 0; i < skipLines; i++ {
		line, err := csvReader.Read()
		if err != nil {
			return err
		}
		csvReader.FieldsPerRecord = 0
		day.HeaderLines = append(day.HeaderLines, line)
	}

	records, err := csvReader.ReadAll()
	if err != nil {
		return err
	}

	for _, record := range records {
		entry, err := rowToRaceEntry(record)
		if err != nil {
			return err
		}

		day.Entries = append(day.Entries, &entry)
	}

	s.days = append(s.days, &day)
	return nil
}

type RaceDay struct {
	Entries     []*RaceEntry
	Path        string
	HeaderLines [][]string
}

func (d *RaceDay) OutputPath() string {
	name := filepath.Base(d.Path)
	nameWOExt := strings.TrimSuffix(name, filepath.Ext(name))
	return filepath.Join(filepath.Dir(d.Path), fmt.Sprintf("processed - %s.csv", nameWOExt))
}

func (d *RaceDay) WriteDay() error {
	file, err := os.OpenFile(d.OutputPath(), os.O_CREATE|os.O_RDWR, os.ModePerm)
	defer file.Close()
	if err != nil {
		return err
	}
	writer := csv.NewWriter(file)

	// Write header lines
	err = writer.WriteAll(d.HeaderLines)
	if err != nil {
		return err
	}

	for _, entry := range d.Entries {
		err := writer.Write(RaceEntryToRow(*entry))
		if err != nil {
			return err
		}
	}

	writer.Flush()
	return nil
}

func NewBibSolver() *BibSolver {
	return &BibSolver{
		bibOverrideCache: map[string]string{},
		UsedBibs:         map[string]interface{}{},
	}
}

type BibSolver struct {
	loanerBibs       []string
	loanerBibIndex   int
	bibOverrideCache map[string]string

	UsedBibs map[string]interface{}
	days     []*RaceDay
}

func (s *BibSolver) IsBibUsed(bib string) bool {
	_, ok := s.UsedBibs[bib]
	return ok
}

func (s *BibSolver) SetBibUsed(bib string) {
	s.UsedBibs[bib] = true
}

func (s *BibSolver) NextLoanerBib() string {
	if s.loanerBibIndex >= len(s.loanerBibs) {
		panic("ran out of loaner bibs")
	}

	loaner := s.loanerBibs[s.loanerBibIndex]
	s.loanerBibIndex++
	return loaner
}

func (s *BibSolver) BibLogic() error {

	// Use all home bibs
	for _, day := range s.days {
		log.Printf("Home Bib Processing %s", filepath.Base(day.Path))

		for _, entry := range day.Entries {
			if entry.HomeBib() != "" {
				s.SetBibUsed(entry.HomeBib())
				log.Printf("\t%s: Home bib allocated %s", entry.LogName(), entry.HomeBib())
			}
		}
	}

	// Use away bibs or loaner, check for interference
	for _, day := range s.days {
		log.Printf("Second Pass Processing %s", filepath.Base(day.Path))
		for _, entry := range day.Entries {
			if entry.HomeBib() == "" {
				// Home Bib is not defined

				// Check if we already assigned this person a bib
				existingOverride := s.bibOverrideCache[entry.PersonKey()]
				if existingOverride != "" {
					log.Printf("\t%s: Using existing overide %s", entry.LogName(), existingOverride)
					entry.SetBib(existingOverride)
				} else {
					if entry.AwayBib() != "" {
						// Away bib is defined
						if s.IsBibUsed(entry.AwayBib()) {
							// Can't use bib, someone else already has it, use loaner
							loaner := s.NextLoanerBib()
							s.SetBibUsed(loaner)
							entry.SetBib(loaner)
							s.bibOverrideCache[entry.PersonKey()] = loaner
							log.Printf("\t%s: Away bib %s is used using loaner %s", entry.LogName(), entry.AwayBib(), loaner)
						} else {
							// Use away bib
							entry.SetBib(entry.AwayBib())
							s.SetBibUsed(entry.AwayBib())
							s.bibOverrideCache[entry.PersonKey()] = entry.AwayBib()
							log.Printf("\t%s: Using Away bib %s", entry.LogName(), entry.AwayBib())
						}
					} else {
						// No bib is defined - use loaner
						loaner := s.NextLoanerBib()
						s.SetBibUsed(loaner)
						entry.SetBib(loaner)
						s.bibOverrideCache[entry.PersonKey()] = loaner
						log.Printf("\t%s: Doesn't have a bib, using loaner %s", entry.LogName(), loaner)
					}
				}
			}
		}
	}

	return nil
}

func (s *BibSolver) WriteOutput() error {
	for _, day := range s.days {
		err := day.WriteDay()
		if err != nil {
			return err
		}
	}

	return nil
}

func main() {
	logFile, err := os.OpenFile("log.txt", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)

	solver := NewBibSolver()
	err = solver.loadLoanerBibs()
	if err != nil {
		panic(err)
	}

	races := []string{"SG Race File 1-30-22.csv", "GS Race File West 1-30-22.csv", "SL Race File West 1-30-22.csv"}
	for _, race := range races {
		err = solver.loadRaceFile(race)
		if err != nil {
			panic(err)
		}
	}

	err = solver.BibLogic()
	if err != nil {
		panic(err)
	}

	err = solver.WriteOutput()
	if err != nil {
		panic(err)
	}

	log.Println("done")
}
