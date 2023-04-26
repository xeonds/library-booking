package main

import (
	"embed"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

//go:embed index.html
var fs embed.FS

// database
type Seat struct {
	gorm.Model
	SeatID            int       `gorm:"uniqueIndex" json:"seat_id"`
	SeatPos           string    `json:"seat_pos"`
	SeatAvailable     bool      `json:"seat_available"`
	SeatBookStartTime time.Time `json:"seat_book_start_time"`
	SeatBookEndTime   time.Time `json:"seat_book_end_time"`
}

var db *gorm.DB
var r *gin.Engine

func InitDB() error {
	var err error
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	err = viper.ReadInConfig()
	if err != nil {
		return err
	}
	user := viper.GetString("db.user")
	pass := viper.GetString("db.pass")
	host := viper.GetString("db.host")
	port := viper.GetString("db.port")
	name := viper.GetString("db.name")
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", user, pass, host, port, name)
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("连接数据库失败：%v", err)
	}
	err = db.AutoMigrate(&Seat{})
	if err != nil {
		return err
	}
	return nil
}

func InitConfig() {
	if _, err := os.Stat("config.yaml"); os.IsNotExist(err) {
		viper.SetDefault("db.user", "user")
		viper.SetDefault("db.pass", "pass")
		viper.SetDefault("db.host", "127.0.0.1")
		viper.SetDefault("db.port", "3306")
		viper.SetDefault("db.name", "dbname")
		viper.SetDefault("server.host", "127.0.0.1")
		viper.SetDefault("server.port", "8080")
		err = viper.WriteConfigAs("config.yaml")
		if err != nil {
			panic(err)
		}
		panic("请修改配置文件后重启程序")
	}
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}
}

func InitRouter() {
	r = gin.Default()
	r.GET("/seats", GetSeats)
	r.POST("/seats/:seatID", SelectSeat)
	r.POST("/seats/random", RandomSeat)
	r.GET("/", func(c *gin.Context) {
		content, err := fs.ReadFile("index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "读取文件失败")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", content)
	})
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
