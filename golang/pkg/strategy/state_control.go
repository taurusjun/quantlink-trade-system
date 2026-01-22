package strategy

import (
	"fmt"
	"time"
)

// StrategyRunState represents the runtime state of a strategy
// Aligned with tbsrc's state control mechanism (m_Active, m_onFlat, m_onExit)
type StrategyRunState int

const (
	// StrategyRunStateActive - Strategy is running normally and can send orders
	// 对应 tbsrc: m_Active=true, m_onFlat=false, m_onExit=false
	StrategyRunStateActive StrategyRunState = iota

	// StrategyRunStatePaused - Strategy is paused due to risk trigger
	// 对应 tbsrc: m_onFlat=true (but can recover)
	StrategyRunStatePaused

	// StrategyRunStateFlattening - Strategy is in the process of closing positions
	// 对应 tbsrc: m_onFlat=true, m_onCancel=true
	StrategyRunStateFlattening

	// StrategyRunStateExiting - Strategy is exiting (cannot recover)
	// 对应 tbsrc: m_onExit=true
	StrategyRunStateExiting

	// StrategyRunStateStopped - Strategy has completely stopped
	// 对应 tbsrc: m_Active=false
	StrategyRunStateStopped
)

// String returns the string representation of StrategyRunState
func (s StrategyRunState) String() string {
	switch s {
	case StrategyRunStateActive:
		return "Active"
	case StrategyRunStatePaused:
		return "Paused"
	case StrategyRunStateFlattening:
		return "Flattening"
	case StrategyRunStateExiting:
		return "Exiting"
	case StrategyRunStateStopped:
		return "Stopped"
	default:
		return "Unknown"
	}
}

// FlattenReason represents the reason for triggering flatten mode
// Used for logging, monitoring, and recovery decisions
type FlattenReason int

const (
	FlattenReasonNone FlattenReason = iota
	// FlattenReasonStopLoss - Stop loss triggered (m_onStopLoss in tbsrc)
	FlattenReasonStopLoss
	// FlattenReasonPriceLimit - Price out of acceptable range (m_onMaxPx in tbsrc)
	FlattenReasonPriceLimit
	// FlattenReasonDeltaLimit - Delta out of acceptable range
	FlattenReasonDeltaLimit
	// FlattenReasonTimeLimit - Trading time limit reached
	FlattenReasonTimeLimit
	// FlattenReasonRejectLimit - Too many order rejections
	FlattenReasonRejectLimit
	// FlattenReasonNewsEvent - News event triggered flatten (m_onNewsFlat in tbsrc)
	FlattenReasonNewsEvent
	// FlattenReasonMaxLoss - Maximum loss limit reached
	FlattenReasonMaxLoss
	// FlattenReasonMaxOrderCount - Maximum order count reached
	FlattenReasonMaxOrderCount
	// FlattenReasonManual - Manual flatten request
	FlattenReasonManual
)

// String returns the string representation of FlattenReason
func (r FlattenReason) String() string {
	switch r {
	case FlattenReasonNone:
		return "None"
	case FlattenReasonStopLoss:
		return "StopLoss"
	case FlattenReasonPriceLimit:
		return "PriceLimit"
	case FlattenReasonDeltaLimit:
		return "DeltaLimit"
	case FlattenReasonTimeLimit:
		return "TimeLimit"
	case FlattenReasonRejectLimit:
		return "RejectLimit"
	case FlattenReasonNewsEvent:
		return "NewsEvent"
	case FlattenReasonMaxLoss:
		return "MaxLoss"
	case FlattenReasonMaxOrderCount:
		return "MaxOrderCount"
	case FlattenReasonManual:
		return "Manual"
	default:
		return "Unknown"
	}
}

// CanRecover returns whether this flatten reason allows automatic recovery
func (r FlattenReason) CanRecover() bool {
	switch r {
	case FlattenReasonStopLoss, FlattenReasonPriceLimit, FlattenReasonDeltaLimit:
		return true // These can recover after conditions normalize
	case FlattenReasonTimeLimit, FlattenReasonMaxLoss, FlattenReasonMaxOrderCount:
		return false // These trigger permanent exit
	default:
		return false
	}
}

// RecoveryCooldown returns the cooldown period before recovery can be attempted
func (r FlattenReason) RecoveryCooldown() time.Duration {
	switch r {
	case FlattenReasonStopLoss:
		return 15 * time.Minute // 对应 tbsrc: 15分钟冷却
	case FlattenReasonPriceLimit:
		return 1 * time.Minute // 对应 tbsrc: 1分钟冷却
	case FlattenReasonDeltaLimit:
		return 5 * time.Minute
	default:
		return 0 // No recovery allowed
	}
}

// StrategyControlState represents the control state of a strategy
// Aligned with tbsrc's state control variables
type StrategyControlState struct {
	// RunState is the current runtime state of the strategy
	RunState StrategyRunState

	// Active indicates if the strategy is activated and can run
	// 对应 tbsrc: m_Active
	// - true: Strategy can send orders (if other conditions allow)
	// - false: Strategy is deactivated and will not send orders
	// Note: In tbsrc, m_Active=false in live mode until manually activated
	Active bool

	// FlattenMode indicates if the strategy should stop sending new orders
	// 对应 tbsrc: m_onFlat
	FlattenMode bool

	// CancelPending indicates if all pending orders should be canceled
	// 对应 tbsrc: m_onCancel
	CancelPending bool

	// ExitRequested indicates if the strategy has been requested to exit
	// 对应 tbsrc: m_onExit (cannot recover)
	ExitRequested bool

	// AggressiveFlat indicates if positions should be closed aggressively
	// (cross the spread to close positions quickly)
	// 对应 tbsrc: m_aggFlat
	AggressiveFlat bool

	// FlattenReason records why flatten mode was triggered
	FlattenReason FlattenReason

	// FlattenTime records when flatten mode was triggered
	FlattenTime time.Time

	// CanRecoverAt records when recovery can be attempted
	// Zero time means no recovery is allowed
	CanRecoverAt time.Time

	// ExitReason records the reason for exit request
	ExitReason string
}

// NewStrategyControlState creates a new StrategyControlState with default values
// autoActivate: if true, strategy starts in Active state (like tbsrc simulation mode)
//               if false, strategy needs manual activation (like tbsrc live mode)
func NewStrategyControlState(autoActivate bool) *StrategyControlState {
	return &StrategyControlState{
		RunState:       StrategyRunStateActive,
		Active:         autoActivate, // 对应 tbsrc: m_Active
		FlattenMode:    false,
		CancelPending:  false,
		ExitRequested:  false,
		AggressiveFlat: false,
		FlattenReason:  FlattenReasonNone,
		FlattenTime:    time.Time{},
		CanRecoverAt:   time.Time{},
		ExitReason:     "",
	}
}

// IsActive returns true if the strategy is in active state
func (scs *StrategyControlState) IsActive() bool {
	return scs.RunState == StrategyRunStateActive
}

// IsStopped returns true if the strategy has stopped
func (scs *StrategyControlState) IsStopped() bool {
	return scs.RunState == StrategyRunStateStopped
}

// CanSendNewOrders returns true if the strategy can send new orders
// 对应 tbsrc: !m_onFlat && m_Active
func (scs *StrategyControlState) CanSendNewOrders() bool {
	return scs.Active &&                          // Must be activated (m_Active)
		scs.RunState == StrategyRunStateActive && // Must be in active state
		!scs.FlattenMode &&                       // Not in flatten mode (m_onFlat)
		!scs.ExitRequested                        // Not exiting (m_onExit)
}

// ShouldCancelOrders returns true if orders should be canceled
// 对应 tbsrc: m_onCancel
func (scs *StrategyControlState) ShouldCancelOrders() bool {
	return scs.CancelPending
}

// CanAttemptRecovery returns true if recovery can be attempted now
func (scs *StrategyControlState) CanAttemptRecovery() bool {
	if scs.ExitRequested {
		return false // Cannot recover if exit is requested
	}

	if !scs.FlattenMode {
		return false // Not in flatten mode
	}

	if !scs.FlattenReason.CanRecover() {
		return false // This reason doesn't allow recovery
	}

	if scs.CanRecoverAt.IsZero() {
		return false // No recovery time set
	}

	return time.Now().After(scs.CanRecoverAt)
}

// Activate activates the strategy (like tbsrc manual activation in live mode)
// 对应 tbsrc: m_Active = true
func (scs *StrategyControlState) Activate() {
	if scs.RunState == StrategyRunStateStopped {
		return // Cannot activate a stopped strategy
	}
	scs.Active = true
}

// Deactivate deactivates the strategy
// 对应 tbsrc: m_Active = false
func (scs *StrategyControlState) Deactivate() {
	scs.Active = false
}

// IsActivated returns true if the strategy is activated
func (scs *StrategyControlState) IsActivated() bool {
	return scs.Active
}

// Reset resets the control state to initial values
func (scs *StrategyControlState) Reset() {
	scs.RunState = StrategyRunStateActive
	scs.Active = true // Reset to activated by default
	scs.FlattenMode = false
	scs.CancelPending = false
	scs.ExitRequested = false
	scs.AggressiveFlat = false
	scs.FlattenReason = FlattenReasonNone
	scs.FlattenTime = time.Time{}
	scs.CanRecoverAt = time.Time{}
	scs.ExitReason = ""
}

// String returns a string representation of the control state
func (scs *StrategyControlState) String() string {
	return fmt.Sprintf("State=%s, Active=%v, Flatten=%v, Cancel=%v, Exit=%v, Aggressive=%v, Reason=%s",
		scs.RunState, scs.Active, scs.FlattenMode, scs.CancelPending, scs.ExitRequested,
		scs.AggressiveFlat, scs.FlattenReason)
}
