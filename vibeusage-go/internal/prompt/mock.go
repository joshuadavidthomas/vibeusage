package prompt

// Mock implements Prompter for testing. Each function field can be set
// to control behavior; if nil, it returns zero values.
type Mock struct {
	InputFunc       func(cfg InputConfig) (string, error)
	ConfirmFunc     func(cfg ConfirmConfig) (bool, error)
	MultiSelectFunc func(cfg MultiSelectConfig) ([]string, error)

	// Call tracking
	InputCalls       []InputConfig
	ConfirmCalls     []ConfirmConfig
	MultiSelectCalls []MultiSelectConfig
}

func (m *Mock) Input(cfg InputConfig) (string, error) {
	m.InputCalls = append(m.InputCalls, cfg)
	if m.InputFunc != nil {
		return m.InputFunc(cfg)
	}
	return "", nil
}

func (m *Mock) Confirm(cfg ConfirmConfig) (bool, error) {
	m.ConfirmCalls = append(m.ConfirmCalls, cfg)
	if m.ConfirmFunc != nil {
		return m.ConfirmFunc(cfg)
	}
	return false, nil
}

func (m *Mock) MultiSelect(cfg MultiSelectConfig) ([]string, error) {
	m.MultiSelectCalls = append(m.MultiSelectCalls, cfg)
	if m.MultiSelectFunc != nil {
		return m.MultiSelectFunc(cfg)
	}
	return nil, nil
}
