package main

import (
	"embed"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

//go:embed index.html
var fs embed.FS

// data models
type Seat struct {
	gorm.Model
	SeatPos string `json:"seat_pos"`
}
type User struct {
	gorm.Model
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}
type Booking struct {
	gorm.Model
	UserID    uint      `json:"user_id"`
	SeatID    uint      `json:"seat_id"`
	User      User      `json:"user" gorm:"foreignKey:UserID"`
	Seat      Seat      `json:"seat" gorm:"foreignKey:SeatID"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

func main() {
	db, _ := gorm.Open(sqlite.Open("data.db"), &gorm.Config{})
	db.AutoMigrate(User{}, Seat{}, Booking{})
	r := gin.Default()
	api := r.Group("/api")
	APIBuilder(func(group *gin.RouterGroup) *gin.RouterGroup {
		group.POST("select", createBooking(db))       // Route to create a booking
		group.GET("available", getAvailableSeats(db)) // Route to fetch available seats
		return group
	})(api, "/booking/")
	AddCRUD[Seat](api, "/seats", db)
	AddCRUD[User](api, "/users", db)
	AddCRUD[Booking](api, "/bookings", db)
	AddStaticFS(r, fs)
	r.Run(":3000")
}

// Function to fetch available seats
func getAvailableSeats(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get time span from request parameters (you may need to adjust this based on your frontend implementation)
		startTime, err := time.Parse(time.RFC3339, c.Query("start_time"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start time"})
			return
		}

		endTime, err := time.Parse(time.RFC3339, c.Query("end_time"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end time"})
			return
		}

		// Query available seats
		var availableSeats []Seat
		if err := db.Raw("SELECT * FROM seats WHERE id NOT IN (SELECT seat_id FROM bookings WHERE ? < end_time AND ? > start_time)", endTime, startTime).Scan(&availableSeats).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch available seats"})
			return
		}

		c.JSON(http.StatusOK, availableSeats)
	}
}

// Function to create a booking
func createBooking(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var booking Booking
		if err := c.ShouldBindJSON(&booking); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Check if seat exists
		var seat Seat
		if err := db.First(&seat, booking.SeatID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid seat ID"})
			return
		}

		// Check if user exists
		var user User
		if err := db.First(&user, booking.UserID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		// Check if booking time slot is available
		var existingBooking Booking
		if err := db.Where("seat_id = ? AND ((start_time <= ? AND end_time >= ?) OR (start_time <= ? AND end_time >= ?))", booking.SeatID, booking.StartTime, booking.StartTime, booking.EndTime, booking.EndTime).First(&existingBooking).Error; err == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Seat already booked for this time slot"})
			return
		}

		// Check if any seat is available in the given time span
		var availableSeat Seat
		if err := db.Raw("SELECT * FROM seats WHERE id = ? AND id NOT IN (SELECT seat_id FROM bookings WHERE ? < end_time AND ? > start_time)", booking.SeatID, booking.EndTime, booking.StartTime).First(&availableSeat).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Seat not available in the time span"})
			return
		}

		// Create booking
		if err := db.Create(&booking).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create booking"})
			return
		}

		c.JSON(http.StatusOK, booking)
	}
}
