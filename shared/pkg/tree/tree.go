// Package tree provides shared utilities for working with metadata file trees.
package tree

import (
	"strings"

	"github.com/fruitsalade/fruitsalade/shared/pkg/models"
)

// FindByPath resolves a path in the metadata tree (recursive).
func FindByPath(root *models.FileNode, path string) *models.FileNode {
	if root == nil {
		return nil
	}
	if root.Path == path {
		return root
	}
	for _, child := range root.Children {
		if found := FindByPath(child, path); found != nil {
			return found
		}
	}
	return nil
}

// FindByID finds a node by its ID in the metadata tree (recursive).
func FindByID(root *models.FileNode, id string) *models.FileNode {
	if root == nil {
		return nil
	}
	if root.ID == id {
		return root
	}
	for _, child := range root.Children {
		if found := FindByID(child, id); found != nil {
			return found
		}
	}
	return nil
}

// CacheID converts a file ID to a cache-safe key (replaces / with _).
func CacheID(id string) string {
	return strings.ReplaceAll(id, "/", "_")
}

// CountNodes counts all nodes in a tree.
func CountNodes(root *models.FileNode) int {
	if root == nil {
		return 0
	}
	count := 1
	for _, child := range root.Children {
		count += CountNodes(child)
	}
	return count
}

// RemoveChild removes a child by name from a parent node.
func RemoveChild(parent *models.FileNode, name string) {
	for i, child := range parent.Children {
		if child.Name == name {
			parent.Children = append(parent.Children[:i], parent.Children[i+1:]...)
			return
		}
	}
}

// BuildChildPath constructs a child path from parent + name.
func BuildChildPath(parentPath, name string) string {
	if parentPath == "/" {
		return "/" + name
	}
	return parentPath + "/" + name
}

// Flatten returns all nodes in a flat map keyed by path.
func Flatten(root *models.FileNode) map[string]*models.FileNode {
	result := make(map[string]*models.FileNode)
	if root == nil {
		return result
	}
	flattenRecursive(root, result)
	return result
}

func flattenRecursive(node *models.FileNode, result map[string]*models.FileNode) {
	result[node.Path] = node
	for _, child := range node.Children {
		flattenRecursive(child, result)
	}
}
