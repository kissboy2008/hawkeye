package api

import (
	"net/http"
	"strconv"
	"time"

	"hawkeye/internal/models"
	"hawkeye/internal/server/alert"
	"hawkeye/internal/server/storage"

	"github.com/gin-gonic/gin"
)

// ---------- 元数据 ----------

// listAlertMetrics 返回可配指标清单，前端按 class 过滤下拉框。
func listAlertMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, alert.MetricsByClass(c.Query("class")))
	}
}

// ---------- 规则（按入口隔离） ----------

// listAlertRulesV2 只返回指定入口的规则，机器和网站不混在一个列表里。
func listAlertRulesV2(db *storage.DB, class string) gin.HandlerFunc {
	return func(c *gin.Context) {
		rules, err := db.GetAllAlertRules()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		out := make([]models.AlertRule, 0, len(rules))
		for _, r := range rules {
			if ruleClass(r) != class {
				continue
			}
			out = append(out, r)
		}
		c.JSON(http.StatusOK, out)
	}
}

// ruleClass 判定规则属于哪个入口（target_class 为空时按指标推断，兼容旧数据）。
func ruleClass(r models.AlertRule) string {
	if r.TargetClass != "" {
		return r.TargetClass
	}
	if r.ScopeType == alert.ClassProbe {
		return alert.ClassProbe
	}
	return alert.ClassOfMetric(r.MetricType)
}

type createRuleV2Body struct {
	Name            string     `json:"name"`
	Description     string     `json:"description"`
	MetricType      string     `json:"metric_type"`
	Operator        string     `json:"operator"`
	Threshold       float64    `json:"threshold"`
	DurationS       int        `json:"duration_s"`
	Enabled         *bool      `json:"enabled"`
	WechatWebhook   string     `json:"wechat_webhook"`
	CooldownS       int        `json:"cooldown_s"`
	RepeatEnabled   bool       `json:"repeat_enabled"`
	RepeatIntervalS int        `json:"repeat_interval_s"`
	SilencedUntil   *time.Time `json:"silenced_until"`

	// 目标清单：建规则时就把每台机器/每个网站列进来，各自可带独立参数
	Targets []struct {
		TargetType string   `json:"target_type"`
		TargetID   int64    `json:"target_id"`
		Enabled    *bool    `json:"enabled"`
		Operator   string   `json:"operator"`
		Threshold  *float64 `json:"threshold"`
		ForS       *int     `json:"for_s"`
	} `json:"targets"`
}

// createAlertRuleV2 建规则：入口决定目标类别，一条规则只能挂同类目标。
func createAlertRuleV2(db *storage.DB, class string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body createRuleV2Body
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if body.Name == "" || body.MetricType == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name 和 metric_type 必填"})
			return
		}
		if got := alert.ClassOfMetric(body.MetricType); got != class {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "指标 " + body.MetricType + " 属于「" + classLabel(got) + "」入口，不能在「" + classLabel(class) + "」入口使用",
			})
			return
		}

		def, _ := alert.MetricDefOf(body.MetricType)
		rule := models.AlertRule{
			Name:            body.Name,
			Description:     body.Description,
			ScopeType:       class, // 保留旧字段，供老接口/老前端读取
			MetricType:      body.MetricType,
			Operator:        body.Operator,
			Threshold:       body.Threshold,
			DurationS:       body.DurationS,
			Enabled:         true,
			WechatWebhook:   body.WechatWebhook,
			CooldownS:       body.CooldownS,
			RepeatEnabled:   body.RepeatEnabled,
			RepeatIntervalS: body.RepeatIntervalS,
			SilencedUntil:   body.SilencedUntil,
			TargetClass:     class,
			Kind:            alert.KindThreshold,
		}
		if def.Availability {
			rule.Kind = alert.KindAvailability
		}
		if rule.Operator == "" {
			rule.Operator = def.DefaultOperator
			if rule.Operator == "" {
				rule.Operator = "gt"
			}
		}
		if rule.Threshold == 0 && def.DefaultThreshold != 0 {
			rule.Threshold = def.DefaultThreshold
		}
		if rule.CooldownS == 0 {
			rule.CooldownS = 60 // v2 默认只做抖动抑制，不再 5 分钟黑洞
		}
		if rule.RepeatIntervalS == 0 && rule.RepeatEnabled {
			rule.RepeatIntervalS = 1800
		}
		if body.Enabled != nil {
			rule.Enabled = *body.Enabled
		}

		id, err := db.CreateAlertRule(&rule)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		rule.ID = id

		if len(body.Targets) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "至少要选一个目标（每台机器/每个网站分别配置）"})
			return
		}

		for _, t := range body.Targets {
			if t.TargetType != "" && t.TargetType != class {
				_ = db.DeleteAlertRule(id)
				c.JSON(http.StatusBadRequest, gin.H{"error": "目标类型与入口不匹配"})
				return
			}
			enabled := true
			if t.Enabled != nil {
				enabled = *t.Enabled
			}
			m := models.AlertRuleTarget{
				RuleID:     id,
				TargetType: class,
				TargetID:   t.TargetID,
				Enabled:    enabled,
				Operator:   t.Operator,
				Threshold:  t.Threshold,
				ForS:       t.ForS,
			}
			if _, err := db.UpsertAlertRuleTarget(&m); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}

		c.JSON(http.StatusCreated, rule)
	}
}

func classLabel(class string) string {
	if class == alert.ClassProbe {
		return "网页监控"
	}
	return "机器监控"
}

// ---------- 详情 / 目标 ----------

// getAlertRuleDetail 返回规则 + 每个目标的实时值/状态，面板那张表直接渲染。
func getAlertRuleDetail(db *storage.DB, engine *alert.Engine) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		rule, err := db.GetAlertRule(id)
		if err != nil || rule == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
			return
		}
		targets, err := db.GetAlertRuleTargets(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		states, _ := db.GetAlertStatesByRule(id)

		for i := range targets {
			t := &targets[i]
			fillTargetLabel(db, t)
			key := t.TargetType + "/" + strconv.FormatInt(t.TargetID, 10)
			if st, ok := states[key]; ok {
				t.Status = st.Status
				t.CurrentValue = st.LastValue
				t.FirstTrueAt = st.FirstTrueAt
			} else {
				t.Status = "ok"
			}
			if engine != nil {
				if v, ok := engine.TargetValue(*rule, *t); ok {
					t.CurrentValue = v
				}
			}
		}
		c.JSON(http.StatusOK, gin.H{"rule": rule, "targets": targets})
	}
}

// fillTargetLabel 补上「这台是谁」，前端表格要显示名字而不是 id。
func fillTargetLabel(db *storage.DB, t *models.AlertRuleTarget) {
	if t.TargetType == alert.ClassProbe {
		if p, err := db.GetProbe(t.TargetID); err == nil && p != nil {
			t.TargetName = p.Name
			t.TargetAddr = p.URL
		}
		return
	}
	if a, err := db.GetAgent(t.TargetID); err == nil && a != nil {
		t.TargetName = a.Name
		t.TargetAddr = a.Address
	}
}

type addTargetsBody struct {
	TargetIDs []int64  `json:"target_ids"`
	Operator  string   `json:"operator"`
	Threshold *float64 `json:"threshold"`
	ForS      *int     `json:"for_s"`
	Enabled   *bool    `json:"enabled"`
}

// addAlertRuleTargets 批量追加目标（向导里的「全选当前全部」走这个）。
func addAlertRuleTargets(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		rule, err := db.GetAlertRule(id)
		if err != nil || rule == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
			return
		}
		var body addTargetsBody
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if len(body.TargetIDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "target_ids 不能为空"})
			return
		}
		class := ruleClass(*rule)
		enabled := true
		if body.Enabled != nil {
			enabled = *body.Enabled
		}
		added := 0
		for _, tid := range body.TargetIDs {
			m := models.AlertRuleTarget{
				RuleID:     id,
				TargetType: class,
				TargetID:   tid,
				Enabled:    enabled,
				Operator:   body.Operator,
				Threshold:  body.Threshold,
				ForS:       body.ForS,
			}
			if _, err := db.UpsertAlertRuleTarget(&m); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			added++
		}
		c.JSON(http.StatusOK, gin.H{"added": added})
	}
}

type patchTargetBody struct {
	Enabled       *bool      `json:"enabled"`
	Operator      *string    `json:"operator"`
	Threshold     *float64   `json:"threshold"`
	ForS          *int       `json:"for_s"`
	SilencedUntil *time.Time `json:"silenced_until"`
	ClearOverride bool       `json:"clear_override"` // true = 回到继承规则默认值
}

// patchAlertRuleTarget 单独改「这一台」的配置。
func patchAlertRuleTarget(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ruleID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		targetType := c.Param("targetType")
		targetID, err := strconv.ParseInt(c.Param("targetId"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid target id"})
			return
		}
		t, err := db.GetAlertRuleTarget(ruleID, targetType, targetID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if t == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "target not found in this rule"})
			return
		}
		var body patchTargetBody
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if body.ClearOverride {
			t.Operator = ""
			t.Threshold = nil
			t.ForS = nil
		}
		if body.Enabled != nil {
			t.Enabled = *body.Enabled
		}
		if body.Operator != nil {
			t.Operator = *body.Operator
		}
		if body.Threshold != nil {
			t.Threshold = body.Threshold
		}
		if body.ForS != nil {
			t.ForS = body.ForS
		}
		if body.SilencedUntil != nil {
			t.SilencedUntil = body.SilencedUntil
		}
		if _, err := db.UpsertAlertRuleTarget(t); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, t)
	}
}

func deleteAlertRuleTarget(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ruleID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		targetType := c.Param("targetType")
		targetID, err := strconv.ParseInt(c.Param("targetId"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid target id"})
			return
		}
		if err := db.DeleteAlertRuleState(ruleID, targetType, targetID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		rows, err := db.GetAlertRuleTargets(ruleID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		for _, t := range rows {
			if t.TargetType == targetType && t.TargetID == targetID {
				if err := db.DeleteAlertRuleTarget(t.ID); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
			}
		}
		c.JSON(http.StatusOK, gin.H{"message": "removed"})
	}
}

// ---------- 健康 ----------

func alertHealth(db *storage.DB, engine *alert.Engine) gin.HandlerFunc {
	return func(c *gin.Context) {
		out := gin.H{}
		if engine != nil {
			out["engine"] = engine.Health()
		}
		if counts, err := db.CountAlertStates(); err == nil {
			out["states"] = counts
		}
		if ob, err := db.CountOutboxByStatus(); err == nil {
			out["outbox"] = ob
		}
		if rules, err := db.GetAllAlertRules(); err == nil {
			out["rules"] = len(rules)
		}
		c.JSON(http.StatusOK, out)
	}
}
