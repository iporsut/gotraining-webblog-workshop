package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql" // import with _ for only register driver
)

var (
	listTmpl = template.Must(template.ParseFiles(filepath.Join("tmpl", "list-post.html")))
	newTmpl  = template.Must(template.ParseFiles(filepath.Join("tmpl", "new-post.html")))
	showTmpl = template.Must(template.ParseFiles(filepath.Join("tmpl", "show-post.html")))
	editTmpl = template.Must(template.ParseFiles(filepath.Join("tmpl", "edit-post.html")))
)

type Post struct {
	ID        int
	Title     string
	Body      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func check(err error) {
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func listPost(ctx context.Context, db *sql.DB) ([]Post, error) {
	qry := "SELECT id, title, body, created_at, updated_at FROM posts"
	stmt, err := db.PrepareContext(ctx, qry)
	if err != nil {
		return nil, err
	}
	rows, err := stmt.QueryContext(ctx)
	if err != nil {
		return nil, err
	}
	var posts []Post
	for rows.Next() {
		var post Post
		err := rows.Scan(&post.ID, &post.Title, &post.Body, &post.CreatedAt, &post.UpdatedAt)
		if err != nil {
			return nil, err
		}
		post.CreatedAt = post.CreatedAt.In(time.Local)
		post.UpdatedAt = post.UpdatedAt.In(time.Local)
		posts = append(posts, post)
	}
	return posts, nil
}

// createPost inserts new post record then return last inserted ID with nil error
// if return non-nil error last inserted ID is zero
func createPost(ctx context.Context, db *sql.DB, post *Post) (int64, error) {
	// LAB
	qry := "INSERT INTO posts(title, body) VALUES (?,?)"
	stmt, err := db.PrepareContext(ctx, qry)
	if err != nil {
		return 0, err
	}
	result, err := stmt.ExecContext(ctx, post.Title, post.Body)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// findPost query posts by ID then scan colum to each post field and return pointer of post
// if return non-nil error pointer is nil
func findPost(ctx context.Context, db *sql.DB, id int) (*Post, error) {
	qry := `SELECT id,
                       title,
                       body,
                       created_at,
                       updated_at
                FROM posts
                WHERE id = ?`
	stmt, err := db.PrepareContext(ctx, qry)
	row := stmt.QueryRowContext(ctx, id)
	var post Post
	err = row.Scan(&post.ID, &post.Title, &post.Body, &post.CreatedAt, &post.UpdatedAt)
	if err != nil {
		return nil, err
	}
	post.CreatedAt = post.CreatedAt.In(time.Local)
	post.UpdatedAt = post.UpdatedAt.In(time.Local)
	return &post, nil
}

func buildUpdateQuery(cur, new *Post) (qry string, args []interface{}, hasUpdate bool) {
	var sets []string
	if cur.Title != new.Title {
		sets = append(sets, "Title = ?")
		args = append(args, new.Title)
	}
	if cur.Body != new.Body {
		sets = append(sets, "Body = ?")
		args = append(args, new.Body)
	}
	if len(sets) == 0 {
		return
	}
	qry = "UPDATE posts SET " + strings.Join(sets, ",") + " WHERE id = ?"
	args = append(args, cur.ID)
	hasUpdate = true
	return
}

func updatePost(ctx context.Context, db *sql.DB, post *Post) error {
	curPost, err := findPost(ctx, db, post.ID)
	if err != nil {
		return err
	}
	qry, args, ok := buildUpdateQuery(curPost, post)
	if !ok {
		return nil
	}
	stmt, err := db.PrepareContext(ctx, qry)
	if err != nil {
		return err
	}
	_, err = stmt.ExecContext(ctx, args...)
	return err
}

func deletePost(ctx context.Context, db *sql.DB, id int) error {
	stmt, err := db.PrepareContext(ctx, "DELETE FROM posts WHERE id = ?")
	if err != nil {
		return err
	}
	_, err = stmt.ExecContext(ctx, id)
	return err
}

// listHandler loads post from DB then send to execute listTmpl
func listHandler(db *sql.DB, w http.ResponseWriter, r *http.Request) error {
	posts, err := listPost(r.Context(), db)
	if err != nil {
		return err
	}
	return listTmpl.Execute(w, posts)
}

// newHandler executes newTmpl
func newHandler(db *sql.DB, w http.ResponseWriter, r *http.Request) error {
	return newTmpl.Execute(w, nil)
}

func createHandler(db *sql.DB, w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return errors.New("only method post for create")
	}
	// populates post
	post := &Post{
		Title: r.FormValue("title"),
		Body:  r.FormValue("body"),
	}
	if id, err := createPost(r.Context(), db, post); err != nil {
		return err
	} else {
		http.Redirect(w, r, fmt.Sprintf("/posts/show/?id=%d", id), http.StatusFound)
		return nil
	}
}

func showHandler(db *sql.DB, w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		return err
	}
	post, err := findPost(r.Context(), db, id)
	if err != nil {
		return err
	}
	return showTmpl.Execute(w, post)
}

// editHandler finds post by id then execute editTmpl with that post
func editHandler(db *sql.DB, w http.ResponseWriter, r *http.Request) error {
	// LAB
	id, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		return err
	}
	post, err := findPost(r.Context(), db, id)
	if err != nil {
		return err
	}
	return editTmpl.Execute(w, post)
}

func updateHandler(db *sql.DB, w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return errors.New("only method post for update")
	}
	// populates post
	id, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		return err
	}
	post := &Post{
		ID:    id,
		Title: r.FormValue("title"),
		Body:  r.FormValue("body"),
	}
	err = updatePost(r.Context(), db, post)
	if err != nil {
		return err
	}
	http.Redirect(w, r, fmt.Sprintf("/posts/show/?id=%d", id), http.StatusFound)
	return nil
}

func deleteHandler(db *sql.DB, w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return errors.New("only method post for delete")
	}
	id, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		return err
	}
	err = deletePost(r.Context(), db, id)
	if err != nil {
		return err
	}
	http.Redirect(w, r, "/", http.StatusFound)
	return nil
}

func wrapError(db *sql.DB,
	h func(*sql.DB, http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(db, w, r); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		}
	}
}

func main() {
	// Open connection to MySQL Database
	db, err := sql.Open("mysql", "htsROjiNf4:zKFKRMrDnQ@tcp(remotemysql.com)/htsROjiNf4?parseTime=true")
	check(err)

	http.HandleFunc("/", wrapError(db, listHandler))
	http.HandleFunc("/posts/new/", wrapError(db, newHandler))
	http.HandleFunc("/posts/create/", wrapError(db, createHandler))
	http.HandleFunc("/posts/show/", wrapError(db, showHandler))
	http.HandleFunc("/posts/edit/", wrapError(db, editHandler))
	http.HandleFunc("/posts/update/", wrapError(db, updateHandler))
	http.HandleFunc("/posts/delete/", wrapError(db, deleteHandler))

	http.ListenAndServe(":8000", nil)
}
