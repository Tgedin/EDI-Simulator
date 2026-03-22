package transformation

// Transformer is a backward-compatibility shim that wraps TransformationEngine.
// It is kept so existing callers compile unchanged during the Step 8 migration.
// Use TransformationEngine directly for all new code.
type Transformer struct {
	engine *TransformationEngine
}

// newTransformer creates a shim-wrapped engine.
func newTransformer() *Transformer {
	return &Transformer{engine: NewTransformationEngine()}
}

// Transform delegates to TransformationEngine.Transform and returns only the
// wire output string, discarding the canonical snapshot.
func (t *Transformer) Transform(sourceFormat, targetFormat, content string) (string, error) {
	if t.engine == nil {
		t.engine = NewTransformationEngine()
	}
	result, err := t.engine.Transform(sourceFormat, targetFormat, content)
	if err != nil {
		return "", err
	}
	return result.Output, nil
}

// SupportedTransformations delegates to TransformationEngine.
func (t *Transformer) SupportedTransformations() []map[string]string {
	if t.engine == nil {
		t.engine = NewTransformationEngine()
	}
	return t.engine.SupportedTransformations()
}
