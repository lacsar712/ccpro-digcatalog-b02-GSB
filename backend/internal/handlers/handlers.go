package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"digcatalog/internal/middleware"
	"digcatalog/internal/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var errFindMissing = errors.New("部分文物不存在")

type Handler struct {
	DB        *gorm.DB
	JWTSecret string
}

func New(db *gorm.DB, jwtSecret string) *Handler {
	return &Handler{DB: db, JWTSecret: jwtSecret}
}

// ---------- Auth ----------

type loginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *Handler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入用户名和密码"})
		return
	}
	var user models.User
	if err := h.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}
	token, err := middleware.GenerateToken(h.JWTSecret, user.ID, user.Username, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
		},
	})
}

func (h *Handler) Me(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"id":       c.MustGet("userId"),
		"username": c.MustGet("username"),
		"role":     c.MustGet("role"),
	})
}

// ---------- Sites ----------

func (h *Handler) ListSites(c *gin.Context) {
	var sites []models.Site
	if err := h.DB.Order("id desc").Find(&sites).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sites)
}

func (h *Handler) GetSite(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var site models.Site
	if err := h.DB.First(&site, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "工地不存在"})
		return
	}
	c.JSON(http.StatusOK, site)
}

func (h *Handler) CreateSite(c *gin.Context) {
	var site models.Site
	if err := c.ShouldBindJSON(&site); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	if site.Name == "" || site.Period == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "名称和时代必填"})
		return
	}
	if err := h.DB.Create(&site).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, site)
}

func (h *Handler) UpdateSite(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var site models.Site
	if err := h.DB.First(&site, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "工地不存在"})
		return
	}
	var req models.Site
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	site.Name = req.Name
	site.Period = req.Period
	site.Latitude = req.Latitude
	site.Longitude = req.Longitude
	site.Manager = req.Manager
	if err := h.DB.Save(&site).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, site)
}

func (h *Handler) DeleteSite(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var count int64
	h.DB.Model(&models.Unit{}).Where("site_id = ?", id).Count(&count)
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该工地下仍有探方，无法删除"})
		return
	}
	if err := h.DB.Delete(&models.Site{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// ---------- Units ----------

func (h *Handler) ListUnits(c *gin.Context) {
	var units []models.Unit
	q := h.DB.Preload("Site").Order("id desc")
	if siteID := c.Query("siteId"); siteID != "" {
		q = q.Where("site_id = ?", siteID)
	}
	if err := q.Find(&units).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, units)
}

func (h *Handler) GetUnit(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var unit models.Unit
	if err := h.DB.Preload("Site").First(&unit, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "探方不存在"})
		return
	}
	c.JSON(http.StatusOK, unit)
}

func (h *Handler) CreateUnit(c *gin.Context) {
	var unit models.Unit
	if err := c.ShouldBindJSON(&unit); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	if unit.SiteID == 0 || unit.Code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "所属工地和编号必填"})
		return
	}
	var site models.Site
	if err := h.DB.First(&site, unit.SiteID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "所属工地不存在"})
		return
	}
	if err := h.DB.Create(&unit).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.DB.Preload("Site").First(&unit, unit.ID)
	c.JSON(http.StatusCreated, unit)
}

func (h *Handler) UpdateUnit(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var unit models.Unit
	if err := h.DB.First(&unit, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "探方不存在"})
		return
	}
	var req models.Unit
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	unit.SiteID = req.SiteID
	unit.Code = req.Code
	unit.DepthMin = req.DepthMin
	unit.DepthMax = req.DepthMax
	unit.StratumDesc = req.StratumDesc
	if err := h.DB.Save(&unit).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.DB.Preload("Site").First(&unit, unit.ID)
	c.JSON(http.StatusOK, unit)
}

func (h *Handler) DeleteUnit(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var count int64
	h.DB.Model(&models.Find{}).Where("unit_id = ?", id).Count(&count)
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该探方下仍有文物，无法删除"})
		return
	}
	if err := h.DB.Delete(&models.Unit{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// ---------- Materials ----------

func (h *Handler) ListMaterials(c *gin.Context) {
	var materials []models.Material
	if err := h.DB.Order("id asc").Find(&materials).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, materials)
}

func (h *Handler) CreateMaterial(c *gin.Context) {
	var m models.Material
	if err := c.ShouldBindJSON(&m); err != nil || m.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "名称必填"})
		return
	}
	if err := h.DB.Create(&m).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, m)
}

func (h *Handler) UpdateMaterial(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var m models.Material
	if err := h.DB.First(&m, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "材质不存在"})
		return
	}
	var req models.Material
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	m.Name = req.Name
	m.Description = req.Description
	if err := h.DB.Save(&m).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, m)
}

func (h *Handler) DeleteMaterial(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.DB.Delete(&models.Material{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// ---------- Finds ----------

type findReq struct {
	UnitID       uint    `json:"unitId"`
	MaterialID   *uint   `json:"materialId"`
	RegisterNo   string  `json:"registerNo"`
	ArtifactType string  `json:"artifactType"`
	MaterialName string  `json:"materialName"`
	Completeness string  `json:"completeness"`
	FindDate     *string `json:"findDate"`
	Description  string  `json:"description"`
	StorageLoc   string  `json:"storageLoc"`
}

func parseDate(s *string) *time.Time {
	if s == nil || *s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return nil
	}
	return &t
}

func (h *Handler) ListFinds(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	applyFilters := func(db *gorm.DB) *gorm.DB {
		if unitID := c.Query("unitId"); unitID != "" {
			db = db.Where("unit_id = ?", unitID)
		}
		if at := c.Query("artifactType"); at != "" {
			db = db.Where("artifact_type = ?", at)
		}
		return db
	}

	var total int64
	if err := applyFilters(h.DB.Model(&models.Find{})).Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var finds []models.Find
	if err := applyFilters(h.DB.Model(&models.Find{})).
		Preload("Unit").Preload("Unit.Site").Preload("Material").
		Order("id desc").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&finds).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items":    finds,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (h *Handler) GetFind(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var find models.Find
	if err := h.DB.Preload("Unit").Preload("Material").First(&find, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文物不存在"})
		return
	}
	c.JSON(http.StatusOK, find)
}

func (h *Handler) applyFindReq(find *models.Find, req *findReq) {
	find.UnitID = req.UnitID
	find.MaterialID = req.MaterialID
	find.RegisterNo = req.RegisterNo
	find.ArtifactType = req.ArtifactType
	find.MaterialName = req.MaterialName
	find.Completeness = req.Completeness
	find.FindDate = parseDate(req.FindDate)
	find.Description = req.Description
	find.StorageLoc = req.StorageLoc
	if find.MaterialID != nil {
		var m models.Material
		if err := h.DB.First(&m, *find.MaterialID).Error; err == nil {
			find.MaterialName = m.Name
		}
	}
}

func (h *Handler) CreateFind(c *gin.Context) {
	var req findReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	if req.UnitID == 0 || req.RegisterNo == "" || req.ArtifactType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "探方、登记号、器物类型必填"})
		return
	}
	var unit models.Unit
	if err := h.DB.First(&unit, req.UnitID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "所属探方不存在"})
		return
	}
	var find models.Find
	h.applyFindReq(&find, &req)
	if err := h.DB.Create(&find).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.DB.Preload("Unit").Preload("Material").First(&find, find.ID)
	c.JSON(http.StatusCreated, find)
}

func (h *Handler) UpdateFind(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var find models.Find
	if err := h.DB.First(&find, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文物不存在"})
		return
	}
	var req findReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	h.applyFindReq(&find, &req)
	if err := h.DB.Save(&find).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.DB.Preload("Unit").Preload("Material").First(&find, find.ID)
	c.JSON(http.StatusOK, find)
}

func (h *Handler) DeleteFind(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.DB.Delete(&models.Find{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

type batchStorageReq struct {
	FindIDs    []uint `json:"findIds"`
	StorageLoc string `json:"storageLoc"`
}

// BatchStorage 在一个事务内把指定文物的存放位置统一更新。
// 任一 id 不存在（或已软删除）则整批回滚。
func (h *Handler) BatchStorage(c *gin.Context) {
	var req batchStorageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数无效"})
		return
	}
	if len(req.FindIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请至少选择一件文物"})
		return
	}
	if strings.TrimSpace(req.StorageLoc) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "存放位置不能为空"})
		return
	}

	// 去掉重复 id，避免数量校验被重复元素干扰
	seen := make(map[uint]struct{}, len(req.FindIDs))
	ids := make([]uint, 0, len(req.FindIDs))
	for _, id := range req.FindIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&models.Find{}).Where("id IN ?", ids).Count(&count).Error; err != nil {
			return err
		}
		// 命中数量与去重后 id 数量不一致，说明有 id 不存在 -> 整批回滚
		if int(count) != len(ids) {
			return errFindMissing
		}
		return tx.Model(&models.Find{}).Where("id IN ?", ids).
			Update("storage_loc", req.StorageLoc).Error
	})
	if err != nil {
		if errors.Is(err, errFindMissing) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "部分文物不存在或已被删除，未做任何修改"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"updated": len(ids)})
}

// ---------- Overview ----------

func (h *Handler) Overview(c *gin.Context) {
	var siteCount, unitCount, findCount int64
	h.DB.Model(&models.Site{}).Count(&siteCount)
	h.DB.Model(&models.Unit{}).Count(&unitCount)
	h.DB.Model(&models.Find{}).Count(&findCount)

	type typeStat struct {
		ArtifactType string `json:"artifactType"`
		Count        int64  `json:"count"`
	}
	var byType []typeStat
	h.DB.Model(&models.Find{}).
		Select("artifact_type as artifact_type, count(*) as count").
		Group("artifact_type").
		Scan(&byType)

	c.JSON(http.StatusOK, gin.H{
		"siteCount": siteCount,
		"unitCount": unitCount,
		"findCount": findCount,
		"byType":    byType,
	})
}
