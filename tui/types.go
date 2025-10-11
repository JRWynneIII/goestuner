package tui

import (
	"fmt"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/jrwynneiii/goestuner/datalink"
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
	SNR                 float64
	AvgSNR              float64
	PeakSNR             float64
}

var overallDecoderStats = DecoderStats{
	false, 0, 0, 0.0, 0.0, 0.0,
}

var DecoderStatsMutex sync.RWMutex

func ReadOverallDecoderStats() DecoderStats {
	DecoderStatsMutex.RLock()
	defer DecoderStatsMutex.RUnlock()
	return overallDecoderStats
}

func WriteOverallDecoderStats(d DecoderStats) {
	DecoderStatsMutex.Lock()
	defer DecoderStatsMutex.Unlock()

	overallDecoderStats = d
}

func ResetChannelAndDecoderStats() {
	WriteOverallDecoderStats(DecoderStats{false, 0, 0, 0.0, 0.0, 0.0})

	channels = []Channel{{0, datalink.VCIDs[0], 0, 0},
		{1, datalink.VCIDs[1], 0, 0},
		{2, datalink.VCIDs[2], 0, 0},
		{6, datalink.VCIDs[6], 0, 0},
		{7, datalink.VCIDs[7], 0, 0},
		{8, datalink.VCIDs[8], 0, 0},
		{9, datalink.VCIDs[9], 0, 0},
		{13, datalink.VCIDs[13], 0, 0},
		{14, datalink.VCIDs[14], 0, 0},
		{15, datalink.VCIDs[15], 0, 0},
		{17, datalink.VCIDs[17], 0, 0},
		{20, datalink.VCIDs[20], 0, 0},
		{21, datalink.VCIDs[21], 0, 0},
		{22, datalink.VCIDs[22], 0, 0},
		{23, datalink.VCIDs[23], 0, 0},
		{24, datalink.VCIDs[24], 0, 0},
		{25, datalink.VCIDs[25], 0, 0},
		{26, datalink.VCIDs[26], 0, 0},
		{30, datalink.VCIDs[30], 0, 0},
		{31, datalink.VCIDs[31], 0, 0},
		{32, datalink.VCIDs[32], 0, 0},
		{60, datalink.VCIDs[60], 0, 0},
		{63, datalink.VCIDs[63], 0, 0}}
}

var channels = []Channel{
	{0, datalink.VCIDs[0], 0, 0},
	{1, datalink.VCIDs[1], 0, 0},
	{2, datalink.VCIDs[2], 0, 0},
	{6, datalink.VCIDs[6], 0, 0},
	{7, datalink.VCIDs[7], 0, 0},
	{8, datalink.VCIDs[8], 0, 0},
	{9, datalink.VCIDs[9], 0, 0},
	{13, datalink.VCIDs[13], 0, 0},
	{14, datalink.VCIDs[14], 0, 0},
	{15, datalink.VCIDs[15], 0, 0},
	{17, datalink.VCIDs[17], 0, 0},
	{20, datalink.VCIDs[20], 0, 0},
	{21, datalink.VCIDs[21], 0, 0},
	{22, datalink.VCIDs[22], 0, 0},
	{23, datalink.VCIDs[23], 0, 0},
	{24, datalink.VCIDs[24], 0, 0},
	{25, datalink.VCIDs[25], 0, 0},
	{26, datalink.VCIDs[26], 0, 0},
	{30, datalink.VCIDs[30], 0, 0},
	{31, datalink.VCIDs[31], 0, 0},
	{32, datalink.VCIDs[32], 0, 0},
	{60, datalink.VCIDs[60], 0, 0},
	{63, datalink.VCIDs[63], 0, 0},
}

var channelsMutex sync.RWMutex

func WriteChannelData(idx int, c Channel) {
	channelsMutex.Lock()
	defer channelsMutex.Unlock()
	channels[idx] = c
}

func ReadChannelData(idx int) Channel {
	channelsMutex.RLock()
	defer channelsMutex.RUnlock()
	return channels[idx]
}

func (l *LockTableData) GetRowCount() int {
	return 6
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
		if !ReadOverallDecoderStats().FrameLock {
			color = tcell.ColorRed
		}
		return tview.NewTableCell(fmt.Sprintf("%v", ReadOverallDecoderStats().FrameLock)).SetTextColor(color)
	case 1:
		if column == 0 {
			return tview.NewTableCell("Total Packets Rx'd:")
		}

		return tview.NewTableCell(fmt.Sprintf("%d", ReadOverallDecoderStats().TotalPackets))
	case 2:
		if column == 0 {
			return tview.NewTableCell("Total Packets Dropped:")
		}

		return tview.NewTableCell(fmt.Sprintf("%d", ReadOverallDecoderStats().TotalDroppedPackets))
	case 3:
		if column == 0 {
			return tview.NewTableCell("SNR:")
		}

		snr := ReadOverallDecoderStats().SNR
		color := ""
		if snr < 1.0 {
			color = "[red]"
		} else {
			color = "[green]"
		}

		return tview.NewTableCell(fmt.Sprintf("%s%f", color, snr))
	case 4:
		if column == 0 {
			return tview.NewTableCell("Average SNR:")
		}

		snr := ReadOverallDecoderStats().AvgSNR
		color := ""
		if snr < 1.0 {
			color = "[red]"
		} else {
			color = "[green]"
		}

		return tview.NewTableCell(fmt.Sprintf("%s%f", color, snr))
	case 5:
		if column == 0 {
			return tview.NewTableCell("Peak SNR:")
		}

		snr := ReadOverallDecoderStats().PeakSNR
		color := ""
		if snr < 1.0 {
			color = "[red]"
		} else {
			color = "[green]"
		}

		return tview.NewTableCell(fmt.Sprintf("%s%f", color, snr))
	default:
		return tview.NewTableCell("ERROR")
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
			return tview.NewTableCell(fmt.Sprintf("[lightskyblue]%d", ReadChannelData(row).ID))
		case 1:
			return tview.NewTableCell(fmt.Sprintf("[white]%s", ReadChannelData(row).Name))
		case 2:
			if ReadChannelData(row).NumPackets == 0 {
				return tview.NewTableCell(fmt.Sprintf("[red]%d", ReadChannelData(row).NumPackets))
			}
			return tview.NewTableCell(fmt.Sprintf("[green]%d", ReadChannelData(row).NumPackets))
		case 3:
			return tview.NewTableCell(fmt.Sprintf("[red]%d", ReadChannelData(row).NumPacketsDropped))
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
