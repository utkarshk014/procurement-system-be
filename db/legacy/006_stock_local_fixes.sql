-- Written by us (sent as ITEM_DELETE_FIX.sql) and installed on live by the DBA on 15 Nov 2026,
-- replacing the sheet's delete_mst_stk_items. Postgres runs it after 005 so local and e2e match live.
--
-- pms.delete_mst_stk_items deletes the item while its row in pms.mst_stk_items_transactions
-- (made by the insert trigger) still points at it, so every delete fails with a foreign key
-- error. This version deletes that row first, but only while it holds no stock and no purchase:
-- once an item has been bought or has an opening stock it must stay, or the history is lost.

CREATE OR REPLACE PROCEDURE pms.delete_mst_stk_items
(
    take_item_cd INTEGER
)
LANGUAGE plpgsql
AS $$
BEGIN

    IF NOT EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_items
        WHERE item_cd = take_item_cd
    ) THEN
        RAISE EXCEPTION 'Stock item record not found';
    END IF;

    IF EXISTS
    (
        SELECT 1
        FROM pms.mst_stk_items_transactions
        WHERE item_cd = take_item_cd
          AND (
                last_purchase_date IS NOT NULL
             OR COALESCE(stk_ob, 0) <> 0
             OR COALESCE(stk_cr, 0) <> 0
             OR COALESCE(stk_db, 0) <> 0
             OR COALESCE(stk_cr_jv, 0) <> 0
             OR COALESCE(stk_db_jv, 0) <> 0
             OR COALESCE(stk_cb, 0) <> 0
          )
    ) THEN
        RAISE EXCEPTION 'Cannot delete! Item has stock or purchase records';
    END IF;

    DELETE FROM pms.mst_stk_items_transactions
    WHERE item_cd = take_item_cd;

    DELETE FROM pms.mst_stk_items
    WHERE item_cd = take_item_cd;

END;
$$;
