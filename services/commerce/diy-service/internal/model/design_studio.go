package model

import (
	"context"
	"github.com/askxuan/common"
)

// UpdateOwned uses a revision compare-and-swap so editing on another device cannot
// silently replace a newer design. Published designs become private when edited.
func (m *defaultDiyDesignModel) UpdateOwned(ctx context.Context, d *DiyDesign, revision int64) error {
	result, err := m.conn.ExecCtx(ctx, `UPDATE askxuan_diy.diy_design SET name=?,design_data=?,total_price=?,description=?,status=?,revision=revision+1,update_time=NOW() WHERE id=? AND user_id=? AND revision=?`, d.Name, d.DesignData, d.TotalPrice, d.Description, d.Status, d.Id, d.UserId, revision)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return common.NewBizError(40914, "设计已在其他页面更新，请重新打开后编辑")
	}
	d.Revision = revision + 1
	return nil
}

func (m *defaultDiyDesignModel) FindStudioList(ctx context.Context, owner, status, keyword string, page, size int) ([]*DiyDesign, int64, error) {
	where := "1=1"
	args := []interface{}{}
	if owner != "" {
		where += " AND user_id=?"
		args = append(args, owner)
	}
	if status != "" {
		where += " AND status=?"
		args = append(args, status)
	}
	if keyword != "" {
		where += " AND name LIKE ?"
		args = append(args, "%"+keyword+"%")
	}
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, "SELECT COUNT(*) FROM askxuan_diy.diy_design WHERE "+where, args...); err != nil {
		return nil, 0, err
	}
	list := []*DiyDesign{}
	args = append(args, (page-1)*size, size)
	err := m.conn.QueryRowsCtx(ctx, &list, "SELECT "+designFields+" FROM askxuan_diy.diy_design WHERE "+where+" ORDER BY update_time DESC,id DESC LIMIT ?,?", args...)
	return list, total, err
}
