package actual

import (
	gotypes "go/types"

	"github.com/nunnatsa/ginkgolinter/internal/gomegainfo"
	"github.com/nunnatsa/ginkgolinter/internal/interfaces"
)

func getAsyncFuncArg(sig *gotypes.Signature) ArgPayload {
	argType := FuncSigArgType
	if sig.Results().Len() == 1 {
		if interfaces.ImplementsError(sig.Results().At(0).Type().Underlying()) {
			argType |= ErrFuncActualArgType | ErrorTypeArgType
		}
	}

	if sig.Params().Len() > 0 {
		arg := sig.Params().At(0).Type()
		if gomegainfo.IsGomegaType(arg) && sig.Results().Len() == 0 {
			argType |= FuncSigArgType | GomegaParamArgType
		}
	}

	if sig.Results().Len() > 1 {
		argType |= FuncSigArgType | MultiRetsArgType
	}

	return &FuncSigArgPayload{argType: argType}
}

type FuncSigArgPayload struct {
	argType ArgType
}

func (f FuncSigArgPayload) ArgType() ArgType {
	return f.argType
}
