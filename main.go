package main

import (
	"archive/zip"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// 全局配置：管理的根目录
var rootDir string

type FileItem struct {
	Name  string
	IsDir bool
}

type PageData struct {
	CurrentPath string
	ParentPath  string
	NotRoot     bool
	Breadcrumbs template.HTML
	Files       []FileItem
}

func main() {
	port := "8080"
	rootDir = "./shared_files"

	if len(os.Args) > 2 {
		port = os.Args[1]
		rootDir = filepath.Clean(os.Args[2])
	}

	_ = os.MkdirAll(rootDir, os.ModePerm)

	// 核心路由绑定
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/upload", handleUpload)
	http.HandleFunc("/mkdir", handleMkdir)
	http.HandleFunc("/download", handleDownload)
	http.HandleFunc("/batch-delete", handleBatchDelete)
	http.HandleFunc("/batch-download", handleBatchDownload)

	fmt.Printf("🚀 现代化文件管理服务已启动！\n监听端口: :%s\n正在管理: %s\n请用浏览器直接访问该 IP 端口管理。\n", port, rootDir)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// 路径安全校验
// 获取安全的绝对物理路径
func getSafePath(reqPath string) string {
	// 1. 统一 Windows 和 Linux 的斜杠
	reqPath = strings.ReplaceAll(reqPath, "\\", "/")
	reqPath = strings.TrimPrefix(reqPath, "/")

	// 2. 获取根目录的绝对路径
	absRootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return rootDir
	}

	// 3. 拼接目标路径并计算绝对路径
	targetPath := filepath.Join(absRootDir, reqPath)
	absTargetPath, err := filepath.Abs(targetPath)
	if err != nil {
		return absRootDir
	}

	// 4. 核心防御：必须保证目标路径是以根目录的绝对路径为前缀
	// 加上路径分隔符可以防止类似 shared_files_secret 绕过 shared_files 的隐患
	if !strings.HasPrefix(absTargetPath, absRootDir) {
		return absRootDir // 如果企图越界，强制将其重定向回根目录
	}

	return absTargetPath
}

// 1. 浏览目录 (动态解析外部的 index.html)
func handleIndex(w http.ResponseWriter, r *http.Request) {
	reqPath := r.URL.Query().Get("path")
	targetFullPath := getSafePath(reqPath)
	relPath, _ := filepath.Rel(rootDir, targetFullPath)
	if relPath == "." {
		relPath = ""
	}

	files, err := os.ReadDir(targetFullPath)
	if err != nil {
		http.Error(w, "无法读取该目录", http.StatusInternalServerError)
		return
	}

	var fileItems []FileItem
	for _, f := range files {
		fileItems = append(fileItems, FileItem{Name: f.Name(), IsDir: f.IsDir()})
	}

	parentPath := ""
	if relPath != "" {
		parentPath = filepath.Dir(relPath)
		if parentPath == "." {
			parentPath = ""
		}
	}

	var bcHTML strings.Builder
	parts := strings.Split(relPath, string(filepath.Separator))
	accumulate := ""
	for _, p := range parts {
		if p != "" {
			accumulate += "/" + p
			bcHTML.WriteString(fmt.Sprintf(` &gt; <a href="/?path=%s">%s</a>`, accumulate, p))
		}
	}

	data := PageData{
		CurrentPath: relPath,
		ParentPath:  parentPath,
		NotRoot:     relPath != "",
		Breadcrumbs: template.HTML(bcHTML.String()),
		Files:       fileItems,
	}

	// 关键修改：直接读取同级目录下的 index.html 文件
	tmpl, err := template.ParseFiles("index.html")
	if err != nil {
		http.Error(w, "未找到 index.html 模板文件，请确保它与可执行文件在同一目录下。", http.StatusInternalServerError)
		return
	}
	_ = tmpl.Execute(w, data)
}

// 2. 多选文件上传
func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// 1. 获取 Multipart Reader
	reader, err := r.MultipartReader()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	currentPath := ""

	// 2. 循环流式读取，直接写到目标盘，避免 Go 产生中间临时文件
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, "解析失败", http.StatusInternalServerError)
			return
		}

		// 记录路径参数
		if part.FormName() == "path" {
			buf := new(strings.Builder)
			_, _ = io.Copy(buf, part)
			currentPath = buf.String()
			continue
		}

		// 处理文件流
		if part.FormName() == "uploadfiles" && part.FileName() != "" {
			targetFullPath := getSafePath(currentPath)
			dst, err := os.Create(filepath.Join(targetFullPath, filepath.Base(part.FileName())))
			if err != nil {
				continue
			}

			// 使用大缓冲区（如 64KB）加速本地传输
			buf := make([]byte, 512*1024)
			_, _ = io.CopyBuffer(dst, part, buf)
			dst.Close()
		}
	}

	http.Redirect(w, r, "/?path="+currentPath, http.StatusSeeOther)
}

// 3. 新建文件夹
func handleMkdir(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	currentPath := r.FormValue("path")
	dirname := r.FormValue("dirname")
	targetFullPath := getSafePath(currentPath)

	if dirname != "" {
		_ = os.MkdirAll(filepath.Join(targetFullPath, filepath.Base(dirname)), os.ModePerm)
	}
	http.Redirect(w, r, "/?path="+currentPath, http.StatusSeeOther)
}

// 4. 批量删除
func handleBatchDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	_ = r.ParseForm()
	currentPath := r.FormValue("path")
	filenames := r.Form["filenames"]
	targetFullPath := getSafePath(currentPath)

	for _, filename := range filenames {
		if filename != "" {
			deletePath := filepath.Join(targetFullPath, filepath.Base(filename))
			_ = os.RemoveAll(deletePath)
		}
	}
	http.Redirect(w, r, "/?path="+currentPath, http.StatusSeeOther)
}

// 5. 批量打包下载
func handleBatchDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	_ = r.ParseForm()
	currentPath := r.FormValue("path")
	filenames := r.Form["filenames"]
	targetFullPath := getSafePath(currentPath)

	if len(filenames) == 0 {
		http.Redirect(w, r, "/?path="+currentPath, http.StatusSeeOther)
		return
	}

	if len(filenames) == 1 {
		singlePath := filepath.Join(targetFullPath, filepath.Base(filenames[0]))
		fi, err := os.Stat(singlePath)
		if err == nil && !fi.IsDir() {
			http.Redirect(w, r, fmt.Sprintf("/download?path=%s/%s", currentPath, filenames[0]), http.StatusSeeOther)
			return
		}
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=batch_download.zip")

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	for _, filename := range filenames {
		sourcePath := filepath.Join(targetFullPath, filepath.Base(filename))

		_ = filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			relPath, err := filepath.Rel(targetFullPath, path)
			if err != nil {
				return err
			}

			header, err := zip.FileInfoHeader(info)
			if err != nil {
				return err
			}

			header.Name = filepath.ToSlash(relPath)
			if info.IsDir() {
				header.Name += "/"
			} else {
				header.Method = zip.Deflate
			}

			writer, err := zipWriter.CreateHeader(header)
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.Copy(writer, file)
			return err
		})
	}
}

// 6. 单文件下载/预览
func handleDownload(w http.ResponseWriter, r *http.Request) {
	reqPath := r.URL.Query().Get("path")
	// 【新增拦截】如果检测到路径企图穿透，直接返回 403 拒绝访问
	if !isPathSafe(reqPath) {
		log.Printf("[警告] 检测到目录穿越攻击！企图访问: %s", reqPath)
		http.Error(w, "非法的访问路径，禁止越权！", http.StatusForbidden)
		return
	}
	targetFullPath := getSafePath(reqPath)

	fi, err := os.Stat(targetFullPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if fi.IsDir() {
		http.Error(w, "这是一个目录，无法直接预览", http.StatusBadRequest)
		return
	}

	ext := strings.ToLower(filepath.Ext(targetFullPath))
	switch ext {
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".gif":
		w.Header().Set("Content-Type", "image/gif")
	case ".txt":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	case ".pdf":
		w.Header().Set("Content-Type", "application/pdf")
	default:
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", filepath.Base(targetFullPath)))
	}

	http.ServeFile(w, r, targetFullPath)
}

// 新增一个判断，用于在关键操作（下载、删除）中直接拦截并报警
func isPathSafe(reqPath string) bool {
	reqPath = strings.ReplaceAll(reqPath, "\\", "/")
	reqPath = strings.TrimPrefix(reqPath, "/")

	absRootDir, _ := filepath.Abs(rootDir)
	targetPath := filepath.Join(absRootDir, reqPath)
	absTargetPath, _ := filepath.Abs(targetPath)

	return strings.HasPrefix(absTargetPath, absRootDir)
}
