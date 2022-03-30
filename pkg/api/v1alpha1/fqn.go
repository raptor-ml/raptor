package v1alpha1

import (
	"fmt"
)

// FQN returns the fully qualified name of the feature.
func (in *Feature) FQN() string {
	return fmt.Sprintf("%s.%s", in.GetName(), in.GetNamespace())
}
