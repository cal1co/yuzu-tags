package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/cal1co/yuzu-feed/middleware"
	"github.com/gin-gonic/gin"
	"github.com/gocql/gocql"
)

var session *gocql.Session

func init() {
	cluster := gocql.NewCluster("127.0.0.1")
	cluster.Keyspace = "user_posts"
	var err error
	session, err = cluster.CreateSession()
	if err != nil {
		panic(err)
	}
}

type PostTags struct {
	Tags   []string
	PostId gocql.UUID
}
type Post struct {
	Id gocql.UUID
}
type Tag struct {
	Value string
}

func main() {
	r := gin.Default()

	r.Use(middleware.RateLimiterMiddleware())

	r.POST("/tag", addTagsToPost)
	r.GET("/postTags/:id", getPostTags)
	r.GET("/taggedPosts/:id", getTaggedPosts)

	go func() {
		if err := r.Run(":8083"); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	log.Println("Shutting down server...")

	log.Println("Server shutdown complete")
}

func addTagsToPost(c *gin.Context) {
	var post PostTags
	if err := c.BindJSON(&post); err != nil {
		fmt.Println(err)
	}
	for i := 0; i < len(post.Tags); i++ {
		fmt.Println(post.Tags[i])
		if err := session.Query(`insert into tags (tag_name, post_id) values (?, ?);`, post.Tags[i], post.PostId).Exec(); err != nil {
			fmt.Println(err)
			c.JSON(http.StatusNotFound, fmt.Sprintf("Sorry, count not add tag '%s' to post with id '%s'", post.Tags[i], post.PostId))
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
	}
	fmt.Println("tags added successfully")
}

func getPostTags(c *gin.Context) {
	var post PostTags
	postId, err := gocql.ParseUUID(c.Param("id"))
	if err != nil {
		fmt.Println(err)
	}
	post.PostId = postId
	iter := session.Query(`SELECT tag_name FROM tags WHERE post_id = ?`, post.PostId).Iter()
	var tag string
	for iter.Scan(&tag) {
		post.Tags = append(post.Tags, tag)
	}
	if err := iter.Close(); err != nil {
		fmt.Println(err)
		c.JSON(http.StatusNotFound, fmt.Sprintf("Sorry, could not fetch tags results for post with id %v", post.PostId))
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.JSON(http.StatusOK, post)
}

func getTaggedPosts(c *gin.Context) {
	tag := c.Param("id")

	iter := session.Query(`SELECT post_id FROM tags WHERE tag_name = ?`, tag).Iter()
	var postId gocql.UUID
	var posts []gocql.UUID
	for iter.Scan(&postId) {
		posts = append(posts, postId)
	}
	if err := iter.Close(); err != nil {
		fmt.Println(err)
		c.JSON(http.StatusNotFound, fmt.Sprintf("Sorry, could not fetch post results for tag %s", tag))
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.JSON(http.StatusOK, posts)
}
