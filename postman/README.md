# Postman: Purchase Approval Flow

Import these files in Postman:

- `inventory-purchase-approval.collection.json`
- `local-cafe.environment.json`

Run the collection in this order:

1. Login Admin
2. Create Inventory Item
3. Create Purchase Request
4. Approve Purchase Request
5. Verify Item Stocks

Before running:

- Start the backend server.
- Update `adminEmail`, `adminPassword`, and `branchId` in the environment.
- Ensure the login user has role `admin` or `manager`.

Expected result:

- The purchase request moves from `pending` to `approved`.
- Stock for the selected branch increases by `purchaseQty`.
- A purchase approval movement is recorded.
