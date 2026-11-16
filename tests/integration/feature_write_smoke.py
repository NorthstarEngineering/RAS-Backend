"""Write smoke for the disposable Docker CI stack on loopback."""
import json
import os
from decimal import Decimal
from urllib.error import HTTPError
from urllib.request import Request, urlopen


def call(port, path, payload=None, method=None, headers=None):
    req = Request(
        f"http://127.0.0.1:{port}/api/v1{path}",
        data=json.dumps(payload).encode() if payload is not None else None,
        method=method,
        headers={"Content-Type": "application/json", **(headers or {})},
    )
    try:
        with urlopen(req, timeout=15) as response:
            return json.loads(response.read() or "null")
    except HTTPError as error:
        raise AssertionError(f"{req.get_method()} {path}: HTTP {error.code}") from None


def authenticate():
    with open(".env", encoding="utf-8") as source:
        fixture = dict(line.strip().split("=", 1) for line in source if "=" in line)
    email = fixture["SEED_CEO_EMAIL"]
    current = fixture["SEED_CEO_PASSWORD"]
    assert email.endswith("@example.test"), "Refusing to use a non-disposable account"
    session = call(9001, "/auth/login", {"email": email, "password": current})
    if session["user"]["must_change_password"]:
        session = call(9001, "/auth/change-password", {
            "current_password": current, "new_password": current + "A9",
        }, headers={"Authorization": "Bearer " + session["access_token"]})
    user = session["user"]
    assert user["role"] == "CEO"
    # Service ports live on loopback. Supply the same identity that the gateway supplies in AWS.
    return user["id"], {
        "Authorization": "Bearer " + session["access_token"],
        "X-User-ID": user["id"], "X-User-Role": user["role"],
    }


def create_branch(api, name, kind):
    return api(9002, "/branches", {
        "name": name, "type": kind, "address": "Disposable CI",
        "official_start_time": "09:00", "late_rate": 0,
    })["id"]


def quantity(api, branch, good):
    return Decimal(api(9010, f"/inventory/items/{good}?branch_id={branch}")["quantity"])


def verify_stock(api, branch, good, amount):
    api(9010, "/inventory/movements", {
        "branch_id": branch, "good_id": good, "type": "IN", "quantity": amount,
    })
    assert quantity(api, branch, good) == amount
    api(9010, f"/inventory/items/{good}/settings?branch_id={branch}",
        {"min_stock": 1, "reorder_level": 3}, method="PATCH")


def verify_transfer(api, source, destination, good):
    transfer = api(9008, "/transfers", {
        "source_branch_id": source, "dest_branch_id": destination,
        "good_id": good, "quantity": 2, "reason": "CI transfer",
    })["data"]
    assert transfer["code"] and transfer["status"] == "PENDING"
    path = f"/transfers/{transfer['id']}"
    api(9008, path + "/approve", {})
    api(9008, path + "/send", {})
    api(9008, path + "/receive", {"received_quantity": 2})
    assert api(9008, path)["data"]["status"] == "RECEIVED"
    assert quantity(api, source, good) == 8
    assert quantity(api, destination, good) == 2


def verify_supply(api, kitchen, destination, good):
    request = api(9004, "/supply/requests", {
        "branch_id": destination, "items": [{"good_id": good, "requested_qty": 2}],
    })
    path = f"/supply/requests/{request['id']}"
    line_id = request["items"][0]["id"]
    api(9004, path + "/approve", {
        "items": [{"item_id": line_id, "approved_qty": 2}],
    }, method="PATCH")
    api(9004, path + "/ship", {}, method="PATCH")
    api(9004, path + "/receive", {
        "items": [{"item_id": line_id, "received_qty": 0}],
    }, method="PATCH")
    assert quantity(api, kitchen, good) == 8
    assert quantity(api, destination, good) == 2, "Zero receipt must not increase stock"
    rows = api(9004, "/supply/requests")
    assert any(row["id"] == request["id"] and row["status"] == "FULFILLED" for row in rows)


def verify_sales(api, branch, actor):
    item = api(9005, "/menu/items", {"sku": "CI-SALE", "name": "CI Sale", "selling_price": 35})
    shift = api(9006, "/sales/shifts/open", {"branch_id": branch, "manager_id": actor})
    api(9006, "/sales/", {
        "shift_id": shift["id"], "channel": "RESTAURANT",
        "items": [{"menu_item_id": item["id"], "quantity": 1}],
    })
    path = f"/sales/shifts/{shift['id']}"
    api(9006, path + "/close", {}, method="PATCH")
    result = api(9006, path + "/summary")
    assert result["status"] == "CLOSED" and Decimal(str(result["gross_sales"])) == 35


def main():
    assert os.environ.get("GITHUB_ACTIONS") == "true", "This script is for disposable CI only"
    actor, identity = authenticate()

    def api(port, path, payload=None, method=None):
        return call(port, path, payload, method, identity)

    source = create_branch(api, "CI Source", "branch")
    destination = create_branch(api, "CI Destination", "branch")
    kitchen = create_branch(api, "CI Kitchen", "central_kitchen")
    good = api(9004, "/inventory/goods", {
        "sku": "CI-GOOD", "name": "CI Good", "unit": "kg", "unit_cost": 3,
    })["id"]
    verify_stock(api, source, good, 10)
    verify_stock(api, kitchen, good, 10)
    print("Stock settings and balances verified", flush=True)
    verify_transfer(api, source, destination, good)
    print("Transfer and both resulting balances verified", flush=True)
    verify_supply(api, kitchen, destination, good)
    print("Supply shipment and zero receipt verified", flush=True)
    verify_sales(api, source, actor)
    print("Sale and closed shift summary verified", flush=True)


if __name__ == "__main__":
    main()
