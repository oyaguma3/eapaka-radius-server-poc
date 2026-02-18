package httputil

import "github.com/gin-gonic/gin"

// WriteError はProblemDetailをGinレスポンスとして書き込む。
func WriteError(c *gin.Context, problem *ProblemDetail) {
	c.Header("Content-Type", ContentType)
	c.JSON(problem.Status, problem)
}

// AbortWithError はProblemDetailをGinレスポンスとして書き込み、リクエスト処理を中断する。
func AbortWithError(c *gin.Context, problem *ProblemDetail) {
	c.Header("Content-Type", ContentType)
	c.AbortWithStatusJSON(problem.Status, problem)
}
