package main

import (
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/iov-one/weave"
	customd "github.com/iov-one/weave-starter-kit/cmd/customd/app"
	"github.com/iov-one/weave/x/countdown"
)

func cmdCountdownStart(input io.Reader, output io.Writer, args []string) error {
	fl := flag.NewFlagSet("", flag.ExitOnError)
	fl.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), `
Start "countdown" extension and save lyrics of Europe's "The final Countdown"
line by line between provided dates
Dates must be provided in format:
YYYY-MM-DD

Default start date is today's date
Default end date is 30 days after the start date

countdown start <start-date> <end-date>

// 
If extension is stopped started before ending date - it will reschedule entering of lines.
If extension is topped and started after the end data, it will start entering lines from the 
begining according to a new schedule.
		`)
		fl.PrintDefaults()
	}
	var (
		err          error
		layout       = "2006-01-02"
		startDefault = time.Now() //.Format("2006-01-02")
		endDefault   = startDefault.Add(time.Hour * 24 * 30)

		_ = fl.String("start", startDefault.Format("2006-01-02"), "Start date")
		_ = fl.String("end", endDefault.Format("2006-01-02"), "End date")
	)
	fl.Parse(args)

	var startDate time.Time
	var endDate time.Time

	if len(args) == 4 {
		startDate, err = time.Parse(layout, args[1])
		if err != nil {
			return err
		}
		endDate, err = time.Parse(layout, args[3])
		if err != nil {
			return err
		}
	}

	if len(args) == 2 {
		startDate, err = time.Parse(layout, args[1])
		if err != nil {
			return err
		}
		endDate = startDate.Add(time.Hour * 24 * 30)
	}

	if len(args) == 0 {
		startDate = time.Now()
		fmt.Println(startDate)
		endDate = startDate.Add(time.Hour * 24 * 30)
	}

	// get hours
	startD := startDate.Format(layout)
	endD := endDate.Format(layout)
	startDate, err = time.Parse(layout, startD)
	if err != nil {
		return err
	}
	endDate, err = time.Parse(layout, endD)
	if err != nil {
		return err
	}

	startDate = startDate.Add(time.Hour*23 + time.Minute*1)
	endDate = endDate.Add(time.Hour*23 + time.Minute*1)

	fmt.Println("Start date:", startDate)
	fmt.Println("End date:", endDate)

	// check if provided start date is before today
	if startDate.Before(time.Now()) {
		flagDie("Start date must be after current time")
	}

	if endDate.Before(time.Now()) || endDate.Equal(time.Now()) {
		flagDie("End date must be after current time")
	}

	// check if end date is before or at start date
	if endDate.Before(startDate) || endDate.Equal(startDate) {
		flagDie("End date must be at least one day later than start date")
	}

	// create new Write instance
	tx := &customd.Tx{
		Sum: &customd.Tx_CountdownCreateMsg{
			CountdownCreateMsg: &countdown.CreateMsg{
				Metadata: &weave.Metadata{Schema: 1},
				Status:   countdown.WriteStatusProcessing,
				Start:    weave.AsUnixTime(startDate),
				End:      weave.AsUnixTime(endDate),
				Read:     int64(0),
				//TaskID:   nil,
			},
		},
	}

	result, err := writeTx(output, tx)
	fmt.Println("result is:", result)
	fmt.Println("error is:", err)

	return err
}

func cmdCountdownStop(input io.Reader, output io.Writer, args []string) error {
	fl := flag.NewFlagSet("", flag.ExitOnError)
	fl.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), `
Stop any running cron processing immediately
countdown-stop
		`)
		fl.PrintDefaults()
	}
	fl.Parse(args)

	// read Write
	tx := &customd.Tx{
		Sum: &customd.Tx_CountdownResetMsg{
			CountdownResetMsg: &countdown.ResetMsg{
				Status: countdown.WriteStatusTerminated,
				End:    weave.AsUnixTime(time.Now()),
			},
		},
	}

	_, err := writeTx(output, tx)
	return err
}
