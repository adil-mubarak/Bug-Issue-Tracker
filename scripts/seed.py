#!/usr/bin/env python3
"""Idempotently seed demo users, project, issues and labels through the gateway."""

import json
import base64
import os
import urllib.error
import urllib.request

BASE = os.environ.get("TRACKER_URL", "http://localhost:8082").rstrip("/")
ADMIN_EMAIL = os.environ.get("SEED_ADMIN_EMAIL", "demo.admin@example.test")
ADMIN_PASSWORD = os.environ.get("SEED_ADMIN_PASSWORD", "demo-admin-password")
MEMBER_EMAIL = os.environ.get("SEED_MEMBER_EMAIL", "demo.member@example.test")
MEMBER_PASSWORD = os.environ.get("SEED_MEMBER_PASSWORD", "demo-member-password")


def request(path, method="GET", token=None, payload=None, expected=(200,)):
    body = None if payload is None else json.dumps(payload).encode()
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    req = urllib.request.Request(BASE + path, data=body, headers=headers, method=method)
    try:
        response = urllib.request.urlopen(req, timeout=10)
    except urllib.error.HTTPError as error:
        if error.code not in expected:
            raise RuntimeError(f"{method} {path}: HTTP {error.code}: {error.read().decode()}") from error
        return error.code, None
    with response:
        data = response.read()
        if response.status not in expected:
            raise RuntimeError(f"{method} {path}: unexpected HTTP {response.status}")
        return response.status, json.loads(data) if data else None


def ensure_user(email, password):
    _, registration = request("/auth/register", "POST", payload={"email": email, "password": password}, expected=(201, 409))
    _, result = request("/auth/login", "POST", payload={"email": email, "password": password})
    token = result["access_token"]
    subject = json.loads(base64.urlsafe_b64decode(token.split(".")[1] + "=="))["sub"]
    return token, registration["id"] if registration else int(subject)


def main():
    admin_token, _ = ensure_user(ADMIN_EMAIL, ADMIN_PASSWORD)
    _, member_id = ensure_user(MEMBER_EMAIL, MEMBER_PASSWORD)
    _, projects = request("/projects", token=admin_token)
    project = next((p for p in projects if p["name"] == "Demo Tracker Project"), None)
    if project is None:
        _, project = request("/projects", "POST", admin_token,
                             {"name": "Demo Tracker Project", "description": "Seeded demonstration workspace"}, (201,))
    project_id = project["id"]
    request(f"/projects/{project_id}/members", "POST", admin_token, {"user_id": member_id}, (201,))
    _, issues = request(f"/issues?project_id={project_id}&limit=50", token=admin_token)
    demos = [
        ("Add regression coverage", "Cover the critical user flows", "high", "todo", "testing"),
        ("Review service readiness", "Verify health and readiness signals", "medium", "in_progress", "operations"),
        ("Publish API guide", "Document the public issue workflows", "low", "done", "documentation"),
    ]
    for title, description, priority, status, label in demos:
        issue = next((item for item in issues if item["title"] == title), None)
        if issue is None:
            _, issue = request("/issues", "POST", admin_token, {
                "project_id": project_id, "title": title, "description": description,
                "priority": priority, "status": status,
            }, (201,))
        request(f"/issues/{issue['id']}/labels", "POST", admin_token, {"name": label}, (201,))
    print(f"Seeded project {project_id}; demo users: {ADMIN_EMAIL}, {MEMBER_EMAIL}")


if __name__ == "__main__":
    main()
