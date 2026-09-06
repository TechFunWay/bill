package billing

import (
	"errors"
	"path"
	"regexp"
	"strings"

	"gorm.io/gorm"
)

const (
	attachmentTypePersonal = "personal"
	attachmentTypeShared   = "shared"
	maxBillAttachments     = 9
)

var billAttachmentPath = regexp.MustCompile(`^/uploads/[0-9]{4}/[0-9]{2}/[0-9]{2}/[a-f0-9-]+\.(jpg|png|gif|webp)$`)

type billAttachmentInput struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

func validateBillAttachments(inputs []billAttachmentInput) ([]BillAttachment, error) {
	if len(inputs) > maxBillAttachments {
		return nil, errors.New("每笔账单最多上传 9 张图片")
	}
	items := make([]BillAttachment, 0, len(inputs))
	seen := make(map[string]bool, len(inputs))
	for index, input := range inputs {
		input.Path = strings.TrimSpace(input.Path)
		if !billAttachmentPath.MatchString(input.Path) || path.Clean(input.Path) != input.Path {
			return nil, errors.New("账单图片路径无效，请重新上传")
		}
		if seen[input.Path] {
			return nil, errors.New("账单图片不能重复")
		}
		seen[input.Path] = true
		name := strings.Map(func(r rune) rune {
			if r < 32 || r == '/' || r == '\\' {
				return -1
			}
			return r
		}, input.Name)
		name = cleanText(name, 160)
		if name == "" {
			name = path.Base(input.Path)
		}
		items = append(items, BillAttachment{Path: input.Path, Name: name, SortOrder: index})
	}
	return items, nil
}

func replaceBillAttachments(db *gorm.DB, transactionType string, transactionID uint, items []BillAttachment) error {
	if err := db.Where("transaction_type = ? AND transaction_id = ?", transactionType, transactionID).Delete(&BillAttachment{}).Error; err != nil {
		return err
	}
	for index := range items {
		items[index].ID = 0
		items[index].TransactionType = transactionType
		items[index].TransactionID = transactionID
		items[index].SortOrder = index
	}
	if len(items) == 0 {
		return nil
	}
	return db.Create(&items).Error
}

func loadBillAttachments(db *gorm.DB, transactionType string, transactionIDs []uint) (map[uint][]BillAttachment, error) {
	result := make(map[uint][]BillAttachment, len(transactionIDs))
	if len(transactionIDs) == 0 {
		return result, nil
	}
	var items []BillAttachment
	if err := db.Where("transaction_type = ? AND transaction_id IN ?", transactionType, transactionIDs).
		Order("transaction_id ASC, sort_order ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	for _, item := range items {
		result[item.TransactionID] = append(result[item.TransactionID], item)
	}
	return result, nil
}

func deleteBillAttachments(db *gorm.DB, transactionType string, transactionID uint) error {
	return db.Where("transaction_type = ? AND transaction_id = ?", transactionType, transactionID).Delete(&BillAttachment{}).Error
}
