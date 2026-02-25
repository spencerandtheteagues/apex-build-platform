package middleware

import (
	"net/http"

	"apex-build/internal/budget"

	"github.com/gin-gonic/gin"
)

// BudgetCheck returns a middleware that verifies budget before expensive operations.
// If no enforcer is provided or the user is not authenticated, the request passes through.
// When budget is exceeded (action=stop), the middleware aborts with 402 Payment Required.
// When budget is approaching the cap (>80%), it sets context values for downstream handlers.
func BudgetCheck(enforcer *budget.BudgetEnforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		if enforcer == nil {
			c.Next()
			return
		}

		userID := c.GetUint("user_id")
		if userID == 0 {
			c.Next()
			return
		}

		buildID := c.Param("id") // build ID from URL if present
		result, err := enforcer.CheckBudget(userID, buildID)
		if err != nil {
			c.Next() // don't block on DB errors
			return
		}

		if !result.Allowed {
			c.JSON(http.StatusPaymentRequired, gin.H{
				"error":       "BUDGET_EXCEEDED",
				"message":     result.Reason,
				"cap_type":    result.CapType,
				"limit_usd":   result.LimitUSD,
				"current_usd": result.CurrentUSD,
			})
			c.Abort()
			return
		}

		// Store warning info for downstream handlers
		if result.WarningPct > 0 {
			c.Set("budget_warning", true)
			c.Set("budget_warning_pct", result.WarningPct)
			c.Set("budget_remaining", result.RemainingUSD)
		}

		c.Next()
	}
}
