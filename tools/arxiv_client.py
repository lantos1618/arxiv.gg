"""Small standard-library client for the arXiv.gg research API.

Set ARXIV_API_KEY in your environment for Deep Search and paper requests.
The descriptive User-Agent identifies this client; it is not required for API access.
"""
import json
import os
import time
import urllib.error
import urllib.parse
import urllib.request


class APIError(RuntimeError):
    def __init__(self, status, message):
        self.status = status
        super().__init__(f"arXiv.gg HTTP {status}: {message}")


class Client:
    def __init__(self, api_key=None, base_url="https://arxiv.gg/api/v1", timeout=25):
        self.api_key = api_key if api_key is not None else os.getenv("ARXIV_API_KEY", "")
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout

    def request(self, path, payload=None, timeout=None):
        headers = {"Accept": "application/json", "User-Agent": "arxivgg-python/1.0 (+https://arxiv.gg)"}
        if self.api_key:
            headers["Authorization"] = "Bearer " + self.api_key
        data = None
        if payload is not None:
            data = json.dumps(payload).encode("utf-8")
            headers["Content-Type"] = "application/json"
        request = urllib.request.Request(self.base_url + path, data=data, headers=headers)
        try:
            with urllib.request.urlopen(request, timeout=timeout or self.timeout) as response:
                body = json.load(response)
        except urllib.error.HTTPError as error:
            # Do not echo request headers or arbitrary edge HTML into agent logs.
            try:
                message = json.loads(error.read(8192)).get("error", "request rejected")
            except (ValueError, UnicodeError):
                message = "request rejected outside the JSON API; check edge security events and API access"
            raise APIError(error.code, message) from None
        if not body.get("success"):
            raise APIError(200, body.get("error", "unsuccessful response"))
        return body["data"]

    def account(self):
        return self.request("/account")

    def research(self, query, max_wait=0, **options):
        """Return immediately by default; optionally poll preparation up to max_wait seconds."""
        path = "/research?" + urllib.parse.urlencode({"q": query, **options})
        result = self.request(path)
        deadline = time.monotonic() + max(0, max_wait)
        while result.get("job") and not result.get("complete"):
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                return result
            job = result["job"]
            time.sleep(min(job.get("retryAfterSeconds", 3), remaining))
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                return result
            status = self.request("/research/status?" + urllib.parse.urlencode({"id": job["id"]}), timeout=min(self.timeout, remaining))
            if status["status"] == "failed":
                raise APIError(503, "query embedding failed; retry later")
            if status["status"] == "ready":
                return self.request(path)
        return result

    def batch(self, queries):
        """Up to four {q, mode, category, since, ...} requests, returned in input order."""
        return self.request("/research/batch", {"queries": queries})

    def request_paper(self, paper_id):
        return self.request("/papers/" + urllib.parse.quote(paper_id, safe="/") + "/request", {})
