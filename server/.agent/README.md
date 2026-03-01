# AI Agent Workspace Guide

This directory manages the planning and context documents for AI coding sessions.

## Directory Structure

* **/context/**
  Active, ongoing tasks and currently relevant markdown documents.

* **/archive/**
  Past history, completed plans, and memos for future improvements. These have low relevance to the current active tasks.

## File Naming Convention

All markdown files use a numeric prefix (e.g., `001_`, `002_`) with the following rules:

* The prefix represents chronological creation order.
* The numbering is a single continuous sequence shared across both `/context` and `/archive`.
* The sequence is strictly increasing.
* Moving a file from `/context` to `/archive` does **not** change its prefix number.

This ensures consistent historical ordering across the entire workspace.
