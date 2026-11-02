package api

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var allowedBgExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true,
}

func uploadBgImage(bgDir string) gin.HandlerFunc {
	return func(c *gin.Context) {
		file, header, err := c.Request.FormFile("image")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请选择图片文件"})
			return
		}
		defer file.Close()

		ext := strings.ToLower(filepath.Ext(header.Filename))
		if !allowedBgExts[ext] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "仅支持 JPG/PNG/WebP/GIF 格式"})
			return
		}

		// 限制文件大小 10MB
		const maxSize = 10 << 20
		if header.Size > maxSize {
			c.JSON(http.StatusBadRequest, gin.H{"error": "文件大小不能超过 10MB"})
			return
		}

		// 确保目录存在
		if err := os.MkdirAll(bgDir, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建目录失败"})
			return
		}

		// 只取文件名部分，杜绝 "../../etc/cron.d/x.png" 这类路径穿越
		// （header.Filename 完全由客户端控制，不可信）
		name := filepath.Base(header.Filename)
		if name == "" || name == "." || name == ".." ||
			strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, "-") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "非法文件名"})
			return
		}

		// 再兜一层：解析成绝对路径后必须仍在 bgDir 内
		absDir, err := filepath.Abs(bgDir)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "目录异常"})
			return
		}
		dst := filepath.Join(absDir, name)
		if filepath.Dir(dst) != absDir {
			c.JSON(http.StatusBadRequest, gin.H{"error": "非法文件名"})
			return
		}

		// O_EXCL：不覆盖已有文件；同名则加时间戳前缀
		out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if os.IsExist(err) {
			name = time.Now().Format("20060102-150405") + "-" + name
			dst = filepath.Join(absDir, name)
			out, err = os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
			return
		}
		defer out.Close()

		if _, err := io.Copy(out, file); err != nil {
			os.Remove(dst)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "写入文件失败"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"filename": name,
			"url":      "/custom_bg/" + name,
		})
	}
}

func listBgImages(bgDir string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 确保目录存在
		os.MkdirAll(bgDir, 0755)

		entries, err := os.ReadDir(bgDir)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"images": []gin.H{}})
			return
		}

		images := make([]gin.H, 0)
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if allowedBgExts[ext] {
				images = append(images, gin.H{
					"filename": entry.Name(),
					"url":      "/custom_bg/" + entry.Name(),
				})
			}
		}

		c.JSON(http.StatusOK, gin.H{"images": images})
	}
}

func deleteBgImage(bgDir string) gin.HandlerFunc {
	return func(c *gin.Context) {
		filename := c.Param("filename")
		// 防止路径穿越
		if strings.Contains(filename, "/") || strings.Contains(filename, "..") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "非法文件名"})
			return
		}
		path := filepath.Join(bgDir, filename)
		if err := os.Remove(path); err != nil {
			if os.IsNotExist(err) {
				c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
			}
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "deleted"})
	}
}
