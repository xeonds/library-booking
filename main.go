package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)


// data models
type ReqeustQuery struct {
	Name string
	Time []string
}
type Seat struct {
	gorm.Model
	SeatID            int       `gorm:"uniqueIndex" json:"seat_id"`
	SeatPos           string    `json:"seat_pos"`
	SeatAvailable     bool      `json:"seat_available"`
	SeatBookStartTime time.Time `json:"seat_book_start_time"`
	SeatBookEndTime   time.Time `json:"seat_book_end_time"`
}

var r *gin.Engine

func create[T any]() func(c *gin.Context) {
	_create := func(model any) error {
		if model == nil {
			return errors.New("model is nil")
		}
		return db.Create(model).Error
	}

	return func(c *gin.Context) {
		var model T
		if err := c.ShouldBindJSON(&model); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error(), "data": model})
			return
		}
		if err := _create(&model); err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "data": model})
			return
		}
		c.JSON(http.StatusCreated, model)
	}
}

func get[T any]() func(c *gin.Context) {
	_get := func(id string) (T, error) {
		var model T
		if err := db.First(&model, id).Error; err != nil {
			return model, err
		}
		return model, nil
	}
	_getAll := func() ([]T, error) {
		var models []T
		if err := db.Find(&models).Error; err != nil {
			return models, err
		}
		return models, nil
	}

	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			models, err := _getAll()
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, models)
		} else {
			model, err := _get(id)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, model)
		}
	}
}

func update[T any]() func(c *gin.Context) {
	_update := func(model any) error {
		if model == nil {
			return errors.New("model is nil")
		}
		return db.Save(model).Error
	}

	return func(c *gin.Context) {
		// id := c.Param("id")
		var model T
		// TODO: check exist
		if err := c.ShouldBindJSON(&model); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := _update(&model); err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, model)
	}
}

func delete[T any]() func(c *gin.Context) {
	_delete := func(model any) error {
		if model == nil {
			return errors.New("model is nil")
		}
		return db.Delete(model).Error
	}
	_deleteById := func(id string, model any) error {
		if model == nil {
			return errors.New("model is nil")
		}
		return db.Where("id = ?", id).Delete(model).Error
	}

	return func(c *gin.Context) {
		id := c.Param("id")
		var model T
		if id == "" {
			// TODO: check exist
			if err := c.ShouldBindJSON(&model); err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if err := _delete(&model); err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		} else {
			if err := _deleteById(id, &model); err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
		c.JSON(http.StatusOK, model)
	}
}

func CRUD[T any](r *gin.RouterGroup, relativePath string) {
	r.POST(relativePath, create[T]())
	r.GET(relativePath+"/:id", get[T]())
	r.GET(relativePath, get[T]())
	r.PUT(relativePath+"/:id", update[T]())
	r.DELETE(relativePath+"/:id", delete[T]())
}

// controllers
func GetSeats(c *gin.Context) {
	var seats []Seat
	if pos := c.Query("seatPos"); pos != "" {
		db = db.Where("seat_pos = ?", pos)
	}
	if available := c.Query("seatAvailable"); available != "" {
		db = db.Where("seat_available = ?", available == "true")
	}
	if startTime := c.Query("startTime"); startTime != "" {
		t, err := time.Parse("2006-01-02 15:04:05", startTime)
		if err != nil {
			c.Error(err)
			return
		}
		db = db.Where("seat_book_end_time < ? OR seat_book_start_time > ?", t, t)
	}
	if endTime := c.Query("endTime"); endTime != "" {
		t, err := time.Parse("2006-01-02 15:04:05", endTime)
		if err != nil {
			c.Error(err)
			return
		}
		db = db.Where("seat_book_end_time < ? OR seat_book_start_time > ?", t, t)
	}
	err := db.Find(&seats).Error
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, seats)
}

func SelectSeat(c *gin.Context) {
	seatID := c.Param("seatID")
	var seat Seat
	err := db.Where("seat_id = ?", seatID).First(&seat).Error
	if err != nil {
		c.Error(err)
		return
	}

	if !seat.SeatAvailable {
		c.JSON(http.StatusBadRequest, gin.H{"message": "座位已被预约"})
		return
	}

	var req struct {
		StartTime time.Time `json:"start_time" binding:"required"`
		EndTime   time.Time `json:"end_time" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	seat.SeatAvailable = false
	seat.SeatBookStartTime = req.StartTime
	seat.SeatBookEndTime = req.EndTime

	if err := db.Save(&seat).Error; err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, seat)
}

func RandomSeat(c *gin.Context) {
	var seats []Seat
	err := db.Find(&seats).Error
	if err != nil {
		c.Error(err)
	}

	// select a random seat
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	randIndex := rng.Intn(len(seats))
	selectedSeat := seats[randIndex]

	// check if the seat is available
	if !selectedSeat.SeatAvailable {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "the selected seat is not available",
		})
		return
	}

	// update the selected seat in the database
	selectedSeat.SeatAvailable = false
	start := time.Now()
	end := start.Add(time.Duration(viper.GetInt("seat.book_duration")) * time.Minute)
	selectedSeat.SeatBookStartTime = start
	selectedSeat.SeatBookEndTime = end
	err = db.Save(&selectedSeat).Error
	if err != nil {
		c.Error(err)
	}

	c.JSON(http.StatusOK, selectedSeat)
}

func init() {
	InitConfig()
	InitRouter()
	InitDB()
}

func main() {
	r.Run(viper.GetString("server.host") + ":" + viper.GetString("server.port"))
}



func InitRouter() {
	r = gin.Default()
	api := r.Group("/api/v1/")
	{
		CRUD[Company](api, "company")
		CRUD[Team](api, "team")
		CRUD[Route](api, "route")
		CRUD[Driver](api, "driver")
		CRUD[RoadManager](api, "road_manager")
		CRUD[Violation](api, "violation")
		CRUD[Vehicle](api, "vehicle")
		// service apis
		api.GET("/seats", get[Seat]())
		api.POST("/seats/:seatID", create[Seat]())
		api.POST("/seats/random", create[Seat]())
		api.POST("query/violation/driver", func(c *gin.Context) {
			var data ReqeustQuery
			c.ShouldBindJSON(&data)
			query := db.Model(&Violation{}).Joins("Vehicle").Joins("Team").Joins("Route").Joins("Driver")
			start, _ := time.Parse(time.RFC3339, data.Time[0])
			end, _ := time.Parse(time.RFC3339, data.Time[1])
			query = query.Where("Driver.Name = ? AND occurred_at BETWEEN ? AND ?", data.Name, start, end)
			var violations []Violation
			query.Find(&violations)
			c.JSON(200, violations)
		})
		api.POST("query/violation/team", func(c *gin.Context) {
			var data ReqeustQuery
			c.ShouldBindJSON(&data)
			query := db.Model(&Violation{}).Joins("Vehicle").Joins("Team").Joins("Route").Joins("Driver")
			start, _ := time.Parse(time.RFC3339, data.Time[0])
			end, _ := time.Parse(time.RFC3339, data.Time[1])
			query = query.Where("Team.Name = ? AND occurred_at BETWEEN ? AND ?", data.Name, start, end)
			var violations []Violation
			query.Find(&violations)
			c.JSON(200, violations)
		})
	}
	r.NoRoute(gin.WrapH(http.FileServer(http.Dir("./dist/"))))
}
