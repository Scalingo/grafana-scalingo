package alerting

import "time"

type EvalHandler interface {
	Eval(evalContext *EvalContext)
}

type Scheduler interface {
	Tick(time time.Time, execQueue chan *Job)
	Update(rules []*Rule)
}

type Notifier interface {
	Notify(evalContext *EvalContext) error
	GetType() string
	NeedsImage() bool
	PassesFilter(rule *Rule) bool

	GetNotifierId() int64
	GetIsDefault() bool
}

type ConditionResult struct {
	Firing      bool
	NoDataFound bool
	Operator    string
	EvalMatches []*EvalMatch
}

type Condition interface {
	Eval(result *EvalContext) (*ConditionResult, error)
}
