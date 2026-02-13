package usecase

type DialogStep string

const (
    StepIdle DialogStep = "idle"

    StepChooseType         DialogStep = "choose_type"
    StepInputMinDiffFact   DialogStep = "input_min_diff_fact"
    StepInputMinDiffPot    DialogStep = "input_min_diff_pot"
    StepInputMaxNotional   DialogStep = "input_max_notional"
)


type WatchType string

const (
    WatchFactOnly      WatchType = "fact_only"
    WatchPotentialOnly WatchType = "potential_only"
    WatchBoth          WatchType = "both"
)

type DialogState struct {
    Step      DialogStep
    WatchType WatchType

    TempMinDiffFact      float64
    TempMinDiffPotential float64
    TempMaxNotional      float64
}
