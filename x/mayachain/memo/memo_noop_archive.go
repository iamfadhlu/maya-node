package mayachain

// ParseNoOpMemoV1 try to parse the memo
func ParseNoOpMemoV1(parts []string) (NoOpMemo, error) {
	if len(parts) > 1 {
		return NewNoOpMemo(parts[1]), nil
	}
	return NewNoOpMemo(""), nil
}
