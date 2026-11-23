#!/usr/bin/env python3
"""Export verified OCI content from an existing backend Actions artifact."""

import argparse
import base64
from concurrent.futures import ThreadPoolExecutor
import hashlib
from http.client import IncompleteRead
import json
import os
from pathlib import Path
import re
import shutil
import tempfile
import time
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode
from urllib.request import Request, build_opener

from ecs_receiver import ArtifactRedirect, SHA, SHA256, download_artifact, validate_image_manifest


INDEX_TYPES = (
    "application/vnd.oci.image.index.v1+json",
    "application/vnd.docker.distribution.manifest.list.v2+json",
)
MANIFEST_TYPES = (
    "application/vnd.oci.image.manifest.v1+json",
    "application/vnd.docker.distribution.manifest.v2+json",
)
ACCEPT_MANIFEST = ", ".join((*INDEX_TYPES, *MANIFEST_TYPES))
METADATA_LIMIT = 8 * 1024**2
DIGEST = re.compile(r"sha256:[0-9a-f]{64}")
RUN_NUMBER = re.compile(r"[1-9][0-9]{0,18}")
REGISTRY = "https://ghcr.io"


def open_url(url, authorization, accept=None):
    request = Request(url)
    # API credentials never follow storage redirects.
    request.add_unredirected_header("Authorization", authorization)
    if accept:
        request.add_header("Accept", accept)
    return build_opener(ArtifactRedirect()).open(request, timeout=45)


def receive_metadata(url, authorization, accept=None):
    with open_url(url, authorization, accept) as response:
        value = response.read(METADATA_LIMIT + 1)
        if response.status != 200 or len(value) > METADATA_LIMIT:
            raise ValueError("Unexpected registry metadata response")
    return value


def export_content(args, stage, actor, job_token):
    request = {"artifact_id": args.artifact_id, "sha256": args.artifact_sha256, "token": job_token}
    source_path = download_artifact("backend", request, stage)
    release = json.loads(source_path.read_bytes())
    validate_image_manifest(release)
    release_pattern = rf"ci-backend-{args.run_id}-[1-9][0-9]*-{args.source[:12]}"
    if not re.fullmatch(release_pattern, release["release"]) or release["history"]["backend"][0] != args.source:
        raise ValueError("Artifact does not match the pinned run and source")

    blobs = stage / "blobs" / "sha256"
    blobs.mkdir(parents=True)
    contents = {}
    downloads = {}
    images = {}
    basic = "Basic " + base64.b64encode(f"{actor}:{job_token}".encode()).decode()

    def manifest_at(repository, digest, authorization, size=None):
        raw = receive_metadata(f"{REGISTRY}/v2/{repository}/manifests/{digest}", authorization, ACCEPT_MANIFEST)
        if (size is not None and len(raw) != size) or hashlib.sha256(raw).hexdigest() != digest[7:]:
            raise ValueError("Registry manifest size or digest mismatch")
        path = blobs / digest[7:]
        if not path.exists():
            path.write_bytes(raw)
        contents[digest] = len(raw)
        value = json.loads(raw)
        if value.get("schemaVersion") != 2:
            raise ValueError("Unsupported registry manifest schema")
        return value

    def verify_descriptor(descriptor):
        digest, size = descriptor["digest"], descriptor["size"]
        if not DIGEST.fullmatch(digest) or type(size) is not int or size < 1:
            raise ValueError("Invalid registry descriptor")
        if digest in contents and contents[digest] != size:
            raise ValueError("Conflicting size for an existing digest")
        return digest, size

    for name, component in sorted(release["components"].items()):
        repository_at, top_digest = component["image"].split("@", 1)
        repository = repository_at.removeprefix("ghcr.io/")
        params = urlencode({"service": "ghcr.io", "scope": f"repository:{repository}:pull"})
        reply = json.loads(receive_metadata(f"{REGISTRY}/token?{params}", basic, "application/json"))
        bearer = reply.get("token") or reply.get("access_token")
        if not isinstance(bearer, str) or not bearer or not bearer.isascii() or any(c.isspace() for c in bearer):
            raise ValueError("Missing registry pull credential")
        authorization = "Bearer " + bearer
        top = manifest_at(repository, top_digest, authorization)
        platform_digest, image = top_digest, top
        if top.get("mediaType") in INDEX_TYPES or (not top.get("mediaType") and "manifests" in top):
            matches = [entry for entry in top["manifests"]
                       if entry.get("platform", {}).get("os") == "linux"
                       and entry.get("platform", {}).get("architecture") == "amd64"
                       and entry.get("mediaType") in MANIFEST_TYPES]
            if len(matches) != 1:
                raise ValueError(f"{name} must have one linux/amd64 image manifest")
            platform_digest, platform_size = verify_descriptor(matches[0])
            image = manifest_at(repository, platform_digest, authorization, platform_size)
        if image.get("mediaType") not in (*MANIFEST_TYPES, None):
            raise ValueError(f"Unsupported image manifest type for {name}")
        config_digest, config_size = verify_descriptor(image["config"])
        if config_size > METADATA_LIMIT:
            raise ValueError(f"Unexpected image configuration size for {name}")
        for descriptor in [image["config"], *image["layers"]]:
            digest, size = verify_descriptor(descriptor)
            if digest not in contents:
                downloads[digest] = (repository, authorization, size)
                contents[digest] = size
        images[name] = {
            "image": component["image"], "linuxAmd64Manifest": platform_digest,
            "config": config_digest, "layers": [layer["digest"] for layer in image["layers"]],
        }

    def download_blob(entry):
        digest, (repository, authorization, size) = entry
        url = f"{REGISTRY}/v2/{repository}/blobs/{digest}"
        for attempt in range(3):
            h = hashlib.sha256()
            received = 0
            try:
                with open_url(url, authorization) as response, (blobs / digest[7:]).open("wb") as file:
                    if response.status != 200:
                        raise ValueError("Unexpected registry blob response")
                    while chunk := response.read(1024**2):
                        received += len(chunk)
                        if received > size:
                            raise ValueError(f"Blob exceeds descriptor size {digest}")
                        h.update(chunk)
                        file.write(chunk)
                if received == size and h.hexdigest() == digest[7:]:
                    return
                if received == size:
                    raise ValueError(f"Blob SHA-256 mismatch {digest}")
                raise OSError("Incomplete registry blob")
            except HTTPError as error:
                if error.code != 429 and error.code < 500:
                    raise
                if attempt == 2:
                    raise
            except (OSError, IncompleteRead):
                if attempt == 2:
                    raise
            time.sleep(2**attempt)

    with ThreadPoolExecutor(max_workers=6) as pool:
        for _ in pool.map(download_blob, downloads.items()):
            pass

    for name, image in images.items():
        configuration = json.loads((blobs / image["config"][7:]).read_bytes())
        labels = configuration.get("config", {}).get("Labels", {})
        if (configuration.get("os") != "linux" or configuration.get("architecture") != "amd64"
                or labels.get("org.opencontainers.image.revision") != args.source
                or labels.get("org.askxuan.binary-sha256") != release["files"][f"payload/{name}/{name}"]):
            raise ValueError(f"Configuration does not match release source or platform for {name}")

    receipt = {
        "schema": 1,
        "source": args.source,
        "release": release["release"],
        "artifact": {"runId": args.run_id, "id": args.artifact_id, "zipSha256": args.artifact_sha256},
        "images": images,
        "contents": contents,
    }
    (stage / "receipt.json").write_text(json.dumps(receipt, indent=2, sort_keys=True) + "\n")
    return len(images), len(contents), sum(contents.values())


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--artifact-id", required=True)
    parser.add_argument("--artifact-sha256", required=True)
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--source", required=True)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    if not all((RUN_NUMBER.fullmatch(args.artifact_id), RUN_NUMBER.fullmatch(args.run_id),
                SHA.fullmatch(args.source), SHA256.fullmatch(args.artifact_sha256))):
        parser.error("Expected pinned numeric run and artifact IDs and hexadecimal digests")
    actor, job_token = os.environ.get("GITHUB_ACTOR"), os.environ.get("GITHUB_TOKEN")
    if not actor or not job_token:
        parser.error("GITHUB_ACTOR and the dedicated job GITHUB_TOKEN are required")
    output = args.output
    if not output.parent.is_dir() or output.exists():
        parser.error("The output must be a fresh directory with an existing parent")
    try:
        with tempfile.TemporaryDirectory(prefix="askxuan-image-export-", dir=output.parent) as work:
            stage = Path(work) / "content"
            stage.mkdir()
            images, count, size = export_content(args, stage, actor, job_token)
            shutil.move(stage, output)
    except HTTPError as error:
        raise SystemExit(f"Image export failed with HTTP {error.code}") from None
    except (OSError, IncompleteRead):
        raise SystemExit("Image export failed during transfer or file IO") from None
    except (KeyError, IndexError, TypeError, json.JSONDecodeError):
        raise SystemExit("Image export encountered unexpected metadata") from None
    except (RuntimeError, ValueError) as error:
        raise SystemExit(f"Image export failed: {error}") from None
    print(f"Verified {images} images in {count} content objects ({size} bytes)")


if __name__ == "__main__":
    main()
