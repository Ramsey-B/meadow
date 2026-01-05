package execution

import (
	"testing"

	"github.com/Ramsey-B/orchid/pkg/expressions"
	"github.com/stretchr/testify/require"
)

func TestExecutionContext_ToMap_AllowsJMESPathAccessToAuthHeaders(t *testing.T) {
	ctx := NewExecutionContext().WithAuth(&AuthContext{
		Headers: map[string]string{
			"Authorization": "Bearer test-token",
		},
	})

	eval := expressions.NewEvaluator()
	got, err := eval.EvaluateString("auth.headers.Authorization", ctx.ToMap())
	require.NoError(t, err)
	require.Equal(t, "Bearer test-token", got)
}


