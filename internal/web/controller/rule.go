package controller

import (
	"net/http"
	"strconv"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/rules"

	"github.com/gin-gonic/gin"
)

// RuleController serves the panel's global routing-rule management.
type RuleController struct {
	BaseController
}

// NewRuleController creates the controller and registers its routes.
func NewRuleController(g *gin.RouterGroup) *RuleController {
	a := &RuleController{}
	a.initRouter(g)
	return a
}

func (a *RuleController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.listRules)
	g.GET("/summary", a.getSummary)
	g.POST("/add", a.addRule)
	g.POST("/update", a.updateRule)
	g.POST("/delete", a.deleteRule)
	g.POST("/reorder", a.reorderRules)

	g.GET("/sources", a.listSources)
	g.POST("/source/analyze", a.analyzeSource)
	g.POST("/source/add", a.addSource)
	g.POST("/source/update", a.updateSource)
	g.POST("/source/delete", a.deleteSource)
	g.POST("/source/reorder", a.reorderSources)
	g.POST("/source/sync", a.syncSource)
	g.GET("/source/entries", a.listSourceEntries)
}

func (a *RuleController) listRules(c *gin.Context) {
	rows, err := rules.ListRules()
	jsonObj(c, rows, err)
}

func (a *RuleController) getSummary(c *gin.Context) {
	summary, err := rules.GetRuleSummary()
	jsonObj(c, summary, err)
}

func (a *RuleController) addRule(c *gin.Context) {
	rule := &model.Rule{Enabled: true}
	if err := c.ShouldBind(rule); err != nil {
		pureJsonMsg(c, http.StatusOK, false, err.Error())
		return
	}
	if err := rules.CreateRule(rule); err != nil {
		pureJsonMsg(c, http.StatusOK, false, err.Error())
		return
	}
	jsonObj(c, rule, nil)
}

func (a *RuleController) updateRule(c *gin.Context) {
	rule := &model.Rule{}
	if err := c.ShouldBind(rule); err != nil {
		pureJsonMsg(c, http.StatusOK, false, err.Error())
		return
	}
	if err := rules.UpdateRule(rule); err != nil {
		pureJsonMsg(c, http.StatusOK, false, err.Error())
		return
	}
	jsonObj(c, rule, nil)
}

func (a *RuleController) deleteRule(c *gin.Context) {
	form := &struct {
		Id int `json:"id" form:"id"`
	}{}
	if err := c.ShouldBind(form); err != nil {
		pureJsonMsg(c, http.StatusOK, false, err.Error())
		return
	}
	jsonMsg(c, "rule deleted", rules.DeleteRule(form.Id))
}

func (a *RuleController) reorderRules(c *gin.Context) {
	form := &struct {
		Ids []int `json:"ids" form:"ids"`
	}{}
	if err := c.ShouldBind(form); err != nil {
		pureJsonMsg(c, http.StatusOK, false, err.Error())
		return
	}
	jsonMsg(c, "rules reordered", rules.ReorderRules(form.Ids))
}

func (a *RuleController) listSources(c *gin.Context) {
	rows, err := rules.ListSources()
	jsonObj(c, rows, err)
}

func (a *RuleController) analyzeSource(c *gin.Context) {
	form := &struct {
		Url string `json:"url" form:"url"`
	}{}
	if err := c.ShouldBind(form); err != nil {
		pureJsonMsg(c, http.StatusOK, false, err.Error())
		return
	}
	result, err := rules.AnalyzeSourceURL(form.Url)
	if err != nil {
		pureJsonMsg(c, http.StatusOK, false, err.Error())
		return
	}
	jsonObj(c, result, nil)
}

func (a *RuleController) addSource(c *gin.Context) {
	source := &model.RuleSource{Enabled: true}
	if err := c.ShouldBind(source); err != nil {
		pureJsonMsg(c, http.StatusOK, false, err.Error())
		return
	}
	if err := rules.CreateSource(source); err != nil {
		pureJsonMsg(c, http.StatusOK, false, err.Error())
		return
	}
	jsonObj(c, source, nil)
}

func (a *RuleController) updateSource(c *gin.Context) {
	source := &model.RuleSource{}
	if err := c.ShouldBind(source); err != nil {
		pureJsonMsg(c, http.StatusOK, false, err.Error())
		return
	}
	if err := rules.UpdateSource(source); err != nil {
		pureJsonMsg(c, http.StatusOK, false, err.Error())
		return
	}
	jsonObj(c, source, nil)
}

func (a *RuleController) deleteSource(c *gin.Context) {
	form := &struct {
		Id int `json:"id" form:"id"`
	}{}
	if err := c.ShouldBind(form); err != nil {
		pureJsonMsg(c, http.StatusOK, false, err.Error())
		return
	}
	jsonMsg(c, "source deleted", rules.DeleteSource(form.Id))
}

func (a *RuleController) reorderSources(c *gin.Context) {
	form := &struct {
		Ids []int `json:"ids" form:"ids"`
	}{}
	if err := c.ShouldBind(form); err != nil {
		pureJsonMsg(c, http.StatusOK, false, err.Error())
		return
	}
	jsonMsg(c, "sources reordered", rules.ReorderSources(form.Ids))
}

func (a *RuleController) syncSource(c *gin.Context) {
	form := &struct {
		Id int `json:"id" form:"id"`
	}{}
	if err := c.ShouldBind(form); err != nil {
		pureJsonMsg(c, http.StatusOK, false, err.Error())
		return
	}
	source, err := rules.GetSource(form.Id)
	if err != nil {
		pureJsonMsg(c, http.StatusOK, false, err.Error())
		return
	}
	if err := rules.SyncSource(source); err != nil {
		pureJsonMsg(c, http.StatusOK, false, err.Error())
		return
	}
	jsonObj(c, source, nil)
}

func (a *RuleController) listSourceEntries(c *gin.Context) {
	sourceId, _ := strconv.Atoi(c.Query("sourceId"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, total, err := rules.ListSourceEntries(sourceId, offset, limit)
	if err != nil {
		pureJsonMsg(c, http.StatusOK, false, err.Error())
		return
	}
	jsonObj(c, gin.H{"entries": rows, "total": total}, nil)
}
