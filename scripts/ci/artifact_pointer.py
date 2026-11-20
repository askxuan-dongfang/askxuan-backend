#!/usr/bin/env python3
"""Write the small authenticated release pointer for the existing SSH receiver."""

import json
import os


print(json.dumps({
    'artifact_id': os.environ['ARTIFACT_ID'],
    'sha256': os.environ['ARTIFACT_SHA256'],
    'commit': os.environ['GITHUB_SHA'],
    'run': os.environ['GITHUB_RUN_ID'],
    'attempt': os.environ['GITHUB_RUN_ATTEMPT'],
    'actor': os.environ['GITHUB_ACTOR'],
    'token': os.environ['GH_TOKEN'],
}))
