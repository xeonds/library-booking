package main

import (
	"embed"
	"fmt"
	"net/http"
	"reflect"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Function to get all records of a given type
func GetAll[T any](db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var records []T
		if err := db.Find(&records).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch records"})
			return
		}
		c.JSON(http.StatusOK, records)
	}
}

// Function to get a single record of a given type by ID
func Get[T any](db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var record T
		id := c.Param("id")
		if err := db.First(&record, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Record not found"})
			return
		}
		c.JSON(http.StatusOK, record)
	}
}

// Function to create a record of a given type
func Create[T any](db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var record T
		if err := c.ShouldBindJSON(&record); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := db.Create(&record).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create record"})
			return
		}
		c.JSON(http.StatusOK, record)
	}
}

// Function to update a record of a given type by ID
func Update[T any](db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var record T
		id := c.Param("id")
		if err := db.First(&record, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Record not found"})
			return
		}
		if err := c.ShouldBindJSON(&record); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := db.Save(&record).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update record"})
			return
		}
		c.JSON(http.StatusOK, record)
	}
}

// Function to delete a record of a given type by ID
func Delete[T any](db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var record T
		id := c.Param("id")
		if err := db.First(&record, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Record not found"})
			return
		}
		if err := db.Delete(&record, id).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete record"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Record deleted successfully"})
	}
}

// Function to find records matching the given struct with pagination
func FindMatchingRecords[T any](db *gorm.DB, returnSingle bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var searchStruct T
		if err := c.ShouldBindJSON(&searchStruct); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		query := PaginatedResults(c)(constructQuery(db, searchStruct))
		// Fetch records
		var records []T
		if returnSingle {
			// If returnSingle is true, return only one record
			if err := query.First(&records).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "No matching record found"})
				return
			}
		} else {
			// Otherwise, return paginated results
			if err := query.Find(&records).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find records"})
				return
			}
		}
		c.JSON(http.StatusOK, records)
	}
}

/*
This functions wraps a set of gin handlers using closure
and compress them to a function

The returned function receives a router and a path
When call the function, it will mount the specfied handlers to the router
*/
func APIBuilder(handlers ...func(*gin.RouterGroup) *gin.RouterGroup) func(gin.IRouter, string) *gin.RouterGroup {
	return func(router gin.IRouter, path string) *gin.RouterGroup {
		group := router.Group(path)
		for _, handler := range handlers {
			group = handler(group)
		}
		return group
	}
}

// Add specfied CRUD routers with generic to a struct with specified path
func AddCRUD[T any](router gin.IRouter, path string, db *gorm.DB) *gin.RouterGroup {
	return APIBuilder(func(group *gin.RouterGroup) *gin.RouterGroup {
		group.GET("", GetAll[T](db))
		group.GET(":id", Get[T](db))
		group.POST("", Create[T](db))
		group.PUT(":id", Update[T](db))
		group.DELETE(":id", Delete[T](db))
		return group
	})(router, path)
}

// Route static folders
func AddStatic(router *gin.Engine, staticFileDir []string) {
	for _, dir := range staticFileDir {
		router.NoRoute(gin.WrapH(http.FileServer(http.Dir(dir))))
	}
}

// Route fs folders
func AddStaticFS(router *gin.Engine, fs embed.FS) {
	router.NoRoute(gin.WrapH(http.FileServer(http.FS(fs))))
}

// Add search handler for a struct
func AddFindAPI[T any](router gin.IRouter, path string, mode string, db *gorm.DB) *gin.RouterGroup {
	return APIBuilder(func(group *gin.RouterGroup) *gin.RouterGroup {
		group.POST("", FindMatchingRecords[T](db, true))
		group.POST("all", FindMatchingRecords[T](db, false))
		return group
	})(router, path)
}

// Recursive function to construct the query based on the fields provided in the struct
func constructQuery[T any](db *gorm.DB, searchStruct T) *gorm.DB {
	query := db
	searchValue := reflect.ValueOf(searchStruct)
	searchType := reflect.TypeOf(searchStruct)
	for i := 0; i < searchValue.NumField(); i++ {
		field := searchType.Field(i)
		value := searchValue.Field(i)

		// Check if the field is a struct
		if value.Kind() == reflect.Struct {
			// Recursively construct query for embedded struct
			query = constructQuery(query, value.Interface())
		} else {
			// Construct query for regular field
			query = query.Where(fmt.Sprintf("%s = ?", field.Name), value.Interface())
		}
	}
	return query
}

// Paginate the results
func PaginatedResults(c *gin.Context) func(*gorm.DB) *gorm.DB {
	type Pagination struct {
		PageSize int
		PageNum  int
	}
	var data Pagination
	pageSize, _ := strconv.Atoi(c.Query("pagesize"))
	pageNum, _ := strconv.Atoi(c.Query("pagenum"))
	switch {
	// handle insufficient pageSize
	case pageSize >= 100:
		data.PageSize = 100
	case pageSize <= 0:
		data.PageSize = 10
	}
	// ensure pageNum(offset) is larger than 1
	if pageNum <= 0 {
		data.PageNum = 1
	}
	return func(db *gorm.DB) *gorm.DB {
		offset := (data.PageNum - 1) * data.PageSize
		return db.Offset(offset).Limit(data.PageSize)
	}
}
