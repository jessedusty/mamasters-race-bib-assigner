package main

import (
	"bytes"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	nativeDialog "github.com/sqweek/dialog"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func tidyUp() {
	fmt.Println("Exited")
}

type runOptions struct {
	LoanerBibFilePath string
	IsNEMS            bool
	RaceDayFilePaths  []string
	OutputDirectory   string
}

func run(app fyne.App, options runOptions) {
	NemsMode = options.IsNEMS

	logFilePath := filepath.Join(options.OutputDirectory, fmt.Sprintf("run-log-%s.txt", time.Now().Format("06-01-02-15:04:05")))
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}

	logBuffer := bytes.Buffer{}

	mw := io.MultiWriter(os.Stdout, logFile, &logBuffer)
	log.SetOutput(mw)

	resultsWindow := app.NewWindow("Results")

	textBody := widget.NewLabel("Run Log")
	windowBody := container.NewBorder(container.NewHBox(widget.NewLabel("Run Log"), layout.NewSpacer(), widget.NewButton("Open output directory", func() {
		oPath, err := url.Parse(fmt.Sprintf("file://%s", strings.ReplaceAll(options.OutputDirectory, "\\", "/")))
		if err != nil || oPath == nil {
			log.Printf("err %v", err)
			return
		}
		_ = app.OpenURL(oPath)
	})), nil, nil, nil,
		container.NewPadded(container.NewVScroll(textBody)))

	resultsWindow.SetContent(windowBody)
	resultsWindow.Show()

	running := true
	defer func() {
		running = false
	}()

	go func() {
		for running {
			textBody.SetText(logBuffer.String())
			time.Sleep(time.Millisecond * 100)
		}
		textBody.SetText(logBuffer.String())
	}()

	solver := NewBibSolver()
	err = solver.loadLoanerBibs(options.LoanerBibFilePath)
	if err != nil {
		log.Printf("error loading loaner bib file: %v", err)
		nativeDialog.Message("error loading loaner bib file: %v", err).Title("Error loading loaner bib file").Error()
		return
	}

	for _, race := range options.RaceDayFilePaths {
		err = solver.loadRaceFile(race)
		if err != nil {
			log.Printf("error loading race file: %v", err)
			nativeDialog.Message("error loading race file: %v", err).Title("Error loading race file").Error()
			return
		}
	}

	err = solver.BibLogic()
	if err != nil {
		log.Printf("error running bib logic: %v", err)
		nativeDialog.Message("error running bib logic: %v", err).Title("Error running bib logic").Error()
		return
	}

	err = solver.WriteOutput(options.OutputDirectory)
	if err != nil {
		log.Printf("error writing output: %v", err)
		nativeDialog.Message("error writing output: %v", err).Title("Error writing output").Error()
		return
	}

	log.Println("done")

}

func main() {

	myApp := app.New()
	myWindow := myApp.NewWindow("Mid-Atlantic Masters Race Bib Assigner")

	homeBib := "Mid-Atlantic"
	homeOrgRadio := widget.NewRadioGroup([]string{"Mid-Atlantic", "New England"}, func(value string) {
		homeBib = value
	})

	homeOrgRadio.Horizontal = true
	homeOrgRadio.SetSelected(homeBib)

	bibEntry := widget.NewEntry()
	bibEntry.SetPlaceHolder("No file selected")
	loanerBibSelection := container.NewBorder(nil, nil, nil, widget.NewButton("Browse",
		func() {
			load, err := nativeDialog.File().Filter("Bib CSV File", "csv").Title("Select Extra Bib CSV").Load()
			if err != nil {
				return
			}

			bibEntry.SetText(load)
		}),
		bibEntry,
	)

	raceDaySelections := []string{}

	raceDayList := container.NewVBox()

	var updateRaceDays func()
	updateRaceDays = func() {
		raceDayList.Hide()
		raceDayList.Objects = nil

		for i, selection := range raceDaySelections {
			raceDayLabel := widget.NewLabel(strings.TrimSuffix(filepath.Base(selection), filepath.Ext(selection)))

			// Copy var b/c it will change on next loop
			index := i
			raceDay := container.NewBorder(nil, nil, nil, widget.NewButton("X", func() {
				// Delete day and re-update
				raceDaySelections = append(raceDaySelections[:index], raceDaySelections[index+1:]...)
				updateRaceDays()

			}), raceDayLabel)

			raceDayList.Add(raceDay)
		}

		raceDayList.Show()
		raceDayList.Refresh()
	}

	updateRaceDays()

	raceDaySelection := container.NewVBox(raceDayList, widget.NewButton("Add New Race Day", func() {
		load, err := nativeDialog.File().Filter("Race CSV File", "csv").Title("Select Race CSV File").Load()
		if err != nil {
			return
		}

		raceDaySelections = append(raceDaySelections, load)
		updateRaceDays()
	}))

	outputDirectoryEntry := widget.NewEntry()
	outputDirectoryChooser := container.NewBorder(nil, nil, nil, widget.NewButton("Choose", func() {
		browse, err := nativeDialog.Directory().Title("Output directory").Browse()
		if err != nil {
			return
		}

		outputDirectoryEntry.SetText(browse)
	}), outputDirectoryEntry)

	mainForm := container.New(layout.NewFormLayout(),
		widget.NewLabel("Home Org"), homeOrgRadio,
		widget.NewLabel("Loaner Bib File"), loanerBibSelection,
		widget.NewLabel("Race Days"), raceDaySelection,
		widget.NewLabel("Output Directory"), outputDirectoryChooser,
	)

	content := container.New(layout.NewVBoxLayout(),
		widget.NewLabel("Mid-Atlantic Masters Race Bib Assigner"),
		mainForm,
		widget.NewButton("Run", func() {
			run(myApp, runOptions{
				IsNEMS:            homeOrgRadio.Selected == "New England",
				LoanerBibFilePath: bibEntry.Text,
				RaceDayFilePaths:  raceDaySelections,
				OutputDirectory:   outputDirectoryEntry.Text,
			})
		}),
	)

	myWindow.SetContent(content)

	myWindow.Show()
	myApp.Run()
	tidyUp()

}
