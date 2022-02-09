package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestRowToRaceEntry(t *testing.T) {
	rowString := "\"12\",\"123\",E5123445,\"\",\"John\",\"Smith\",\"1900\",\"M\",\"Team\",\"11/21/2021 19:49:11\",\"Alpine Coach (w/ Official)Alpine OfficialAlpine Master (w/ requirements)\",\"\",\"Epic  Other.. \",\n"

	cells, err := csv.NewReader(strings.NewReader(rowString)).Read()
	if err != nil {
		panic(err)
	}

	entry, err := rowToRaceEntry(cells)
	if err != nil {
		panic(err)
	}

	JSON, err := json.MarshalIndent(&entry, " ", " ")
	if err != nil {
		panic(err)
	}

	fmt.Println(string(JSON))
}
