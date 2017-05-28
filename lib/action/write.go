package action

import (
	"github.com/mithrandie/csvq/lib/cmd"
	"github.com/mithrandie/csvq/lib/output"
	"github.com/mithrandie/csvq/lib/query"
)

func Write(input string) error {
	results, err := query.Execute(input)
	if err != nil {
		return err
	}

	flags := cmd.GetFlags()

	var out string
	for _, result := range results {
		s, err := output.Encode(result)
		if err != nil {
			return err
		}
		out += s
	}

	err = output.Write(flags.OutFile, out)
	if err != nil {
		return err
	}

	return nil
}