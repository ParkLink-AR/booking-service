package entity_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/example/bookings-service/internal/domain/entity"
)

func TestGenerateVerificationCode_IsFourNumericDigits(t *testing.T) {
	for i := 0; i < 100; i++ {
		code, err := entity.GenerateVerificationCode()
		require.NoError(t, err)
		assert.Len(t, code, 4)
		for _, r := range code {
			assert.True(t, r >= '0' && r <= '9', "code %q contains a non-digit", code)
		}
	}
}
