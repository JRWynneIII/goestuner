package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gdamore/tcell/v2"
	"github.com/jrwynneiii/goestuner/config"
	"github.com/jrwynneiii/goestuner/decode"
	"github.com/jrwynneiii/goestuner/demod"
	"github.com/navidys/tvxwidgets"
	"github.com/rivo/tview"
)

type ChannelTableData struct {
	tview.TableContentReadOnly
}

type LockTableData struct {
	tview.TableContentReadOnly
}

type Channel struct {
	ID                int
	Name              string
	NumPackets        int
	NumPacketsDropped int
}

type DecoderStats struct {
	FrameLock           bool
	TotalPackets        int
	TotalDroppedPackets int
}

var overallDecoderStats = DecoderStats{
	false, 0, 0,
}

var channels = []Channel{
	{0, decode.VCIDs[0], 0, 0},
	{1, decode.VCIDs[1], 0, 0},
	{2, decode.VCIDs[2], 0, 0},
	{6, decode.VCIDs[6], 0, 0},
	{7, decode.VCIDs[7], 0, 0},
	{8, decode.VCIDs[8], 0, 0},
	{9, decode.VCIDs[9], 0, 0},
	{13, decode.VCIDs[13], 0, 0},
	{14, decode.VCIDs[14], 0, 0},
	{15, decode.VCIDs[15], 0, 0},
	{17, decode.VCIDs[17], 0, 0},
	{20, decode.VCIDs[20], 0, 0},
	{21, decode.VCIDs[21], 0, 0},
	{22, decode.VCIDs[22], 0, 0},
	{23, decode.VCIDs[23], 0, 0},
	{24, decode.VCIDs[24], 0, 0},
	{25, decode.VCIDs[25], 0, 0},
	{26, decode.VCIDs[26], 0, 0},
	{30, decode.VCIDs[30], 0, 0},
	{31, decode.VCIDs[31], 0, 0},
	{32, decode.VCIDs[32], 0, 0},
	{60, decode.VCIDs[60], 0, 0},
	{63, decode.VCIDs[63], 0, 0},
}

func (l *LockTableData) GetRowCount() int {
	return 3
}

func (l *LockTableData) GetColumnCount() int {
	return 2
}

func (l *LockTableData) GetCell(row, column int) *tview.TableCell {
	switch row {
	case 0:
		if column == 0 {
			return tview.NewTableCell("Frame lock:")
		}

		color := tcell.ColorGreen
		if !overallDecoderStats.FrameLock {
			color = tcell.ColorRed
		}
		return tview.NewTableCell(fmt.Sprintf("%v", overallDecoderStats.FrameLock)).SetTextColor(color)
	case 1:
		if column == 0 {
			return tview.NewTableCell("Total Packets Rx'd:")
		}

		return tview.NewTableCell(fmt.Sprintf("%d", overallDecoderStats.TotalPackets))
	case 2:
		if column == 0 {
			return tview.NewTableCell("Total Packets Dropped:")
		}

		return tview.NewTableCell(fmt.Sprintf("%d", overallDecoderStats.TotalDroppedPackets))
	}
	return tview.NewTableCell("ERROR")
}

func (d *ChannelTableData) GetRowCount() int {
	return len(channels)
}

func (d *ChannelTableData) GetColumnCount() int {
	return 4
}

func (c *ChannelTableData) GetCell(row, column int) *tview.TableCell {
	if row != 0 {
		switch column {
		case 0:
			return tview.NewTableCell(fmt.Sprintf("[lightskyblue]%d", channels[row].ID))
		case 1:
			return tview.NewTableCell(fmt.Sprintf("[white]%s", channels[row].Name))
		case 2:
			if channels[row].NumPackets == 0 {
				return tview.NewTableCell(fmt.Sprintf("[red]%d", channels[row].NumPackets))
			}
			return tview.NewTableCell(fmt.Sprintf("[green]%d", channels[row].NumPackets))
		case 3:
			return tview.NewTableCell(fmt.Sprintf("[red]%d", channels[row].NumPacketsDropped))
		}
	} else {
		switch column {
		case 0:
			return tview.NewTableCell("[lightskyblue]Channel ID ")
		case 1:
			return tview.NewTableCell("[white]Channel Name ")
		case 2:
			return tview.NewTableCell("[green]Packets RX'd ")
		case 3:
			return tview.NewTableCell("[red]Packets Dropped")
		}

	}
	return tview.NewTableCell("ERROR")

}

var LogOut *tview.TextView

func StartUI(decoder *decode.Decoder, demodulator *demod.Demodulator, enableFFT bool, tuiConf config.TuiConf) {
	app := tview.NewApplication()

	LogOut = tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(true)

	channelData := &ChannelTableData{}
	lockData := &LockTableData{}
	channelStats := tview.NewTable().SetContent(channelData)
	lockTable := tview.NewTable().SetContent(lockData)

	signalPlot := tvxwidgets.NewPlot()
	signalPlot.SetLineColor([]tcell.Color{tcell.ColorLightSkyBlue})
	signalPlot.SetMarker(tvxwidgets.PlotMarkerBraille)

	signalGauge := tvxwidgets.NewUtilModeGauge()
	signalGauge.SetLabel("Signal Strength:             ")
	signalGauge.SetLabelColor(tcell.ColorLightSkyBlue)
	signalGauge.SetWarnPercentage(99)
	signalGauge.SetCritPercentage(100)
	signalGauge.SetEmptyColor(tcell.ColorBlack)
	signalGauge.SetBorder(false)

	berGauge := tvxwidgets.NewUtilModeGauge()
	berGauge.SetLabel("Viterbi Error Rate:          ")
	berGauge.SetLabelColor(tcell.ColorLightSkyBlue)
	berGauge.SetWarnPercentage(tuiConf.VitWarnPct)
	berGauge.SetCritPercentage(tuiConf.VitCritPct)
	berGauge.SetEmptyColor(tcell.ColorBlack)
	berGauge.SetBorder(false)

	rsCorrectionsGauge := tvxwidgets.NewUtilModeGauge()
	rsCorrectionsGauge.SetLabel("Reed-Soloman Corrections:    ")
	rsCorrectionsGauge.SetLabelColor(tcell.ColorLightSkyBlue)
	rsCorrectionsGauge.SetWarnPercentage(tuiConf.RsWarnPct)
	rsCorrectionsGauge.SetCritPercentage(tuiConf.RsCritPct)
	rsCorrectionsGauge.SetEmptyColor(tcell.ColorBlack)
	rsCorrectionsGauge.SetBorder(false)

	gaugeBox := tview.NewFlex()
	gaugeBox.SetDirection(tview.FlexRow)
	gaugeBox.AddItem(signalGauge, 0, 1, false)
	gaugeBox.AddItem(berGauge, 0, 1, false)
	gaugeBox.AddItem(rsCorrectionsGauge, 0, 1, false)
	gaugeBox.SetTitle("Signal Stats")
	gaugeBox.SetBorder(true)

	LogOut.SetChangedFunc(func() {
		LogOut.ScrollToEnd()
		app.Draw()
	})

	LogOut.SetBorder(true).SetTitle("Log Output")
	log.SetOutput(LogOut)
	channelStats.SetSelectable(false, false).SetBorder(true).SetTitle("Per-Channel Stats")
	lockTable.SetSelectable(false, false).SetBorder(false)

	decoderStats := tview.NewFlex().SetDirection(tview.FlexRow)
	decoderStats.AddItem(tview.NewBox(), 0, 1, false)
	decoderStats.AddItem(lockTable, 0, 1, false)
	decoderStats.AddItem(tview.NewBox(), 0, 1, false)
	decoderStats.SetBorder(true)
	decoderStats.SetTitle("Decoder Status")

	signalPlot.SetBorder(true)
	signalPlot.SetTitle("Signal")

	page := tview.NewFlex().SetDirection(tview.FlexColumn)

	leftCol := tview.NewFlex().SetDirection(tview.FlexRow)
	leftCol.AddItem(channelStats, 0, 3, false)
	leftCol.AddItem(decoderStats, 0, 1, false)

	rightCol := tview.NewFlex().SetDirection(tview.FlexRow)
	rightCol.AddItem(gaugeBox, 0, 4, false)
	if enableFFT {
		rightCol.AddItem(signalPlot, 0, 2, false)
	}

	if tuiConf.EnableLogOutput {
		rightCol.AddItem(LogOut, 0, 2, false)
	}

	page.AddItem(leftCol, 0, 2, false)
	page.AddItem(rightCol, 0, 5, false)

	//Update Stats
	go func() {
		for {
			// Gather stats from decoder
			frameLock := decoder.FrameLock
			ber := decoder.Viterbi.GetPercentBER()
			sigquality := decoder.SigQuality
			averageRsCorrections := decoder.AverageRsCorrections
			packetsPerChannel := decoder.RxPacketsPerChannel
			droppedPacketsPerChannel := decoder.DroppedPacketsPerChannel
			totalFrames := decoder.TotalFramesProcessed
			RSCorrectionPercent := (averageRsCorrections / float32(totalFrames)) * 100

			// Update channel stats
			var totalPacketsDropped int
			for idx, channel := range channels {
				channel.NumPackets = packetsPerChannel[channel.ID]
				channel.NumPacketsDropped = droppedPacketsPerChannel[channel.ID]
				channels[idx] = channel
				totalPacketsDropped += channel.NumPacketsDropped
			}
			//Update gauges
			signalGauge.SetValue(float64(sigquality))
			berGauge.SetValue(float64(ber))
			rsCorrectionsGauge.SetValue(float64(RSCorrectionPercent))

			//Update decoder stats
			overallDecoderStats.FrameLock = frameLock
			overallDecoderStats.TotalPackets = totalFrames
			overallDecoderStats.TotalDroppedPackets = totalPacketsDropped

			//Update signal plot
			if len(demodulator.CurrentFFT) > 0 {
				var bins []float64
				for _, val := range demodulator.CurrentFFT {
					bins = append(bins, val)
				}
				log.Infof("FFT Len:", len(bins))
				signalPlot.SetData([][]float64{bins})
			}

			app.Draw()
			//Sleep half a second
			//TODO: maybe make this a config value, or ability to set on the fly?
			time.Sleep(time.Duration(tuiConf.RefreshMs) * time.Millisecond)

		}
	}()

	if err := app.SetRoot(page, true).EnableMouse(true).Run(); err != nil {
		log.Fatalf("Could not start UI: %v", err)
	}
}
