package progressbar

import (
	"fmt"

	"github.com/cheggaaa/pb/v3"
)

func SpinnerTemplate(text string, num int) pb.ProgressBarTemplate {
	return pb.ProgressBarTemplate(fmt.Sprintf(
		`{{ green "%s %d:" }} {{ (cycle . "←" "↖" "↑" "↗" "→" "↘" "↓" "↙" ) }} {{speed . }}`,
		text, num))
}

func ProgressTemplate(text string) pb.ProgressBarTemplate {
	return pb.ProgressBarTemplate(
		fmt.Sprintf(
			`{{ blue "%s:" }} {{ counters . }} {{ bar . "[" "█" (cycle . "█") "▒" "]" }} {{ percent . }}`,
			text))
}

func Template(text string) pb.ProgressBarTemplate {
	return pb.ProgressBarTemplate(text)
}
