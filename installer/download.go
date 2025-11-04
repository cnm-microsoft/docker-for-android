package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// CDN 和服务器地址
	cdnURL    = "https://fw.kspeeder.com/binary/docker-for-android"
	serverURL = "https://fw.koolcenter.com/binary/docker-for-android"
)

// CreateHTTPClient 创建统一的 HTTP 客户端
// 使用 120 秒超时的自定义 Transport
func CreateHTTPClient() *http.Client {
	transport := CreateTimeoutTransport(120 * time.Second)
	return &http.Client{
		Transport: CreateLogTransport(transport),
		Timeout:   10 * time.Minute,
	}
}

// downloadFile 下载文件并验证 SHA256
func downloadFile(client *http.Client, destPath, filename, expectedSHA256 string) error {
	// 首先检查本地文件是否存在且有效
	localPath := filepath.Join(localInstallDir, filename)
	if fileExists(localPath) {
		fmt.Printf("   尝试使用本地文件: %s\n", localPath)
		
		// 复制本地文件到目标位置
		if err := copyFile(localPath, destPath); err != nil {
			fmt.Printf("   ✗ 本地文件复制失败: %v\n", err)
		} else {
			// 验证 SHA256
			if expectedSHA256 != "" {
				if err := verifySHA256(destPath, expectedSHA256); err != nil {
					fmt.Printf("   ✗ 本地文件 SHA256 验证失败: %v\n", err)
					os.Remove(destPath)
				} else {
					fmt.Printf("   ✓ 本地文件 SHA256 验证通过\n")
					return nil
				}
			} else {
				fmt.Printf("   ✓ 使用本地文件成功\n")
				return nil
			}
		}
	}

	urls := []string{
		fmt.Sprintf("%s/%s", cdnURL, filename),
		fmt.Sprintf("%s/%s", serverURL, filename),
	}

	var lastErr error
	for i, url := range urls {
		source := "CDN"
		if i > 0 {
			source = "服务器"
		}
		fmt.Printf("   尝试从%s下载...\n", source)

		err := downloadFromURL(client, url, destPath)
		if err != nil {
			lastErr = err
			fmt.Printf("   ✗ 下载失败: %v\n", err)
			continue
		}

		// 验证 SHA256
		if expectedSHA256 != "" {
			if err := verifySHA256(destPath, expectedSHA256); err != nil {
				lastErr = err
				fmt.Printf("   ✗ SHA256 验证失败: %v\n", err)
				os.Remove(destPath)
				continue
			}
			fmt.Printf("   ✓ SHA256 验证通过\n")
		}

		return nil
	}

	return fmt.Errorf("所有下载源均失败: %v", lastErr)
}

// downloadFromURL 从指定 URL 下载文件
func downloadFromURL(client *http.Client, url, destPath string) error {
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP 请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP 状态码: %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}
	defer out.Close()

	// 创建进度显示
	var written int64
	contentLength := resp.ContentLength

	if contentLength > 0 {
		// 带进度条的下载
		buf := make([]byte, 32*1024)
		lastPrintTime := time.Now()

		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				_, writeErr := out.Write(buf[:n])
				if writeErr != nil {
					return fmt.Errorf("写入文件失败: %v", writeErr)
				}
				written += int64(n)

				// 每秒更新一次进度
				if time.Since(lastPrintTime) >= time.Second {
					progress := float64(written) / float64(contentLength) * 100
					fmt.Printf("   进度: %.1f%% (%d/%d MB)\r",
						progress,
						written/(1024*1024),
						contentLength/(1024*1024))
					lastPrintTime = time.Now()
				}
			}
			if err != nil {
				if err == io.EOF {
					fmt.Printf("   进度: 100.0%% (%d/%d MB)\n",
						written/(1024*1024),
						contentLength/(1024*1024))
					break
				}
				return fmt.Errorf("读取数据失败: %v", err)
			}
		}
	} else {
		// 无法获取大小时，简单复制
		written, err = io.Copy(out, resp.Body)
		if err != nil {
			return fmt.Errorf("下载失败: %v", err)
		}
		fmt.Printf("   下载完成: %d MB\n", written/(1024*1024))
	}

	return nil
}

// verifySHA256 验证文件的 SHA256
func verifySHA256(filePath, expectedHash string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %v", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("计算哈希失败: %v", err)
	}

	actualHash := hex.EncodeToString(hash.Sum(nil))
	if actualHash != strings.ToLower(expectedHash) {
		return fmt.Errorf("SHA256 不匹配 (期望: %s, 实际: %s)", expectedHash, actualHash)
	}

	return nil
}
