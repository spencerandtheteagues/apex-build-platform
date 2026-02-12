// APEX.BUILD Code Comments Component
// Inline code comments and threads for collaboration (Replit parity feature)

import React, { useState, useEffect, useCallback, useRef } from 'react'
import { cn } from '@/lib/utils'
import apiService from '@/services/api'
import { CodeComment, CodeCommentThread, File } from '@/types'
import { Button, Badge, Avatar, Card, Loading } from '@/components/ui'
import {
  MessageSquare,
  Reply,
  Check,
  CheckCircle2,
  RotateCcw,
  Trash2,
  Edit2,
  X,
  Send,
  SmilePlus,
  ChevronDown,
  ChevronUp,
  AlertCircle,
} from 'lucide-react'

// Common emoji reactions
const EMOJI_REACTIONS = ['thumbs_up', 'thumbs_down', 'heart', 'rocket', 'eyes', 'thinking']
const EMOJI_MAP: Record<string, string> = {
  thumbs_up: '\u{1F44D}',
  thumbs_down: '\u{1F44E}',
  heart: '\u{2764}\u{FE0F}',
  rocket: '\u{1F680}',
  eyes: '\u{1F440}',
  thinking: '\u{1F914}',
}

export interface CodeCommentsProps {
  file: File
  projectId: number
  currentUserId: number
  currentUsername: string
  onCommentClick?: (line: number) => void
  className?: string
}

export interface CommentMarker {
  lineNumber: number
  threadId: string
  commentCount: number
  isResolved: boolean
}

export const CodeComments: React.FC<CodeCommentsProps> = ({
  file,
  projectId,
  currentUserId,
  currentUsername,
  onCommentClick,
  className,
}) => {
  const [comments, setComments] = useState<CodeComment[]>([])
  const [markers, setMarkers] = useState<CommentMarker[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [selectedThread, setSelectedThread] = useState<string | null>(null)
  const [showResolved, setShowResolved] = useState(false)
  const [isCreating, setIsCreating] = useState(false)
  const [newCommentLine, setNewCommentLine] = useState<number | null>(null)
  const [newCommentContent, setNewCommentContent] = useState('')
  const [replyingTo, setReplyingTo] = useState<number | null>(null)
  const [replyContent, setReplyContent] = useState('')
  const [editingComment, setEditingComment] = useState<number | null>(null)
  const [editContent, setEditContent] = useState('')
  const [showEmojiPicker, setShowEmojiPicker] = useState<number | null>(null)

  // Fetch comments when file changes
  useEffect(() => {
    if (file?.id) {
      fetchComments()
    }
  }, [file?.id, showResolved]) // eslint-disable-line react-hooks/exhaustive-deps -- fetch helper is intentionally local for readability.

  // Set up WebSocket listener for real-time updates
  // Note: When WebSocket comment events are implemented, they should trigger fetchComments()
  // For now, we poll on file changes and rely on manual refresh
  useEffect(() => {
    // Future WebSocket integration placeholder
    // websocketService can be extended to support comment events
    // when the backend implements WebSocket broadcasting for comments
  }, [file?.id])

  const fetchComments = async () => {
    if (!file?.id) return

    setLoading(true)
    setError(null)

    try {
      const response = await apiService.getFileComments(file.id, {
        include_resolved: showResolved,
      })
      setComments(response.comments)
      updateMarkers(response.comments)
    } catch (err: any) {
      setError(err.message || 'Failed to load comments')
      console.error('Failed to fetch comments:', err)
    } finally {
      setLoading(false)
    }
  }

  const updateMarkers = (commentList: CodeComment[]) => {
    // Group comments by thread and create markers
    const threadMap = new Map<string, CodeComment[]>()

    commentList.forEach(comment => {
      const existing = threadMap.get(comment.thread_id) || []
      threadMap.set(comment.thread_id, [...existing, comment])
    })

    const newMarkers: CommentMarker[] = []
    threadMap.forEach((threadComments, threadId) => {
      const rootComment = threadComments.find(c => !c.parent_id) || threadComments[0]
      newMarkers.push({
        lineNumber: rootComment.start_line,
        threadId,
        commentCount: threadComments.length + (threadComments[0]?.replies?.length || 0),
        isResolved: rootComment.is_resolved,
      })
    })

    setMarkers(newMarkers.sort((a, b) => a.lineNumber - b.lineNumber))
  }

  const handleCreateComment = async () => {
    if (!newCommentContent.trim() || newCommentLine === null) return

    setIsCreating(true)
    try {
      await apiService.createCodeComment({
        file_id: file.id,
        project_id: projectId,
        start_line: newCommentLine,
        end_line: newCommentLine,
        content: newCommentContent.trim(),
      })

      setNewCommentContent('')
      setNewCommentLine(null)
      await fetchComments()
    } catch (err: any) {
      setError(err.message || 'Failed to create comment')
    } finally {
      setIsCreating(false)
    }
  }

  const handleReply = async (parentId: number, threadId: string) => {
    if (!replyContent.trim()) return

    setIsCreating(true)
    try {
      const parentComment = comments.find(c => c.id === parentId)
      await apiService.createCodeComment({
        file_id: file.id,
        project_id: projectId,
        start_line: parentComment?.start_line || 1,
        end_line: parentComment?.end_line || 1,
        content: replyContent.trim(),
        parent_id: parentId,
        thread_id: threadId,
      })

      setReplyContent('')
      setReplyingTo(null)
      await fetchComments()
    } catch (err: any) {
      setError(err.message || 'Failed to post reply')
    } finally {
      setIsCreating(false)
    }
  }

  const handleUpdateComment = async (commentId: number) => {
    if (!editContent.trim()) return

    try {
      await apiService.updateCodeComment(commentId, {
        content: editContent.trim(),
      })

      setEditContent('')
      setEditingComment(null)
      await fetchComments()
    } catch (err: any) {
      setError(err.message || 'Failed to update comment')
    }
  }

  const handleDeleteComment = async (commentId: number) => {
    if (!confirm('Are you sure you want to delete this comment?')) return

    try {
      await apiService.deleteCodeComment(commentId)
      await fetchComments()

      // If we deleted from the selected thread, check if thread still exists
      const deletedComment = comments.find(c => c.id === commentId)
      if (deletedComment && selectedThread === deletedComment.thread_id) {
        const remainingInThread = comments.filter(
          c => c.thread_id === deletedComment.thread_id && c.id !== commentId
        )
        if (remainingInThread.length === 0) {
          setSelectedThread(null)
        }
      }
    } catch (err: any) {
      setError(err.message || 'Failed to delete comment')
    }
  }

  const handleResolveThread = async (commentId: number) => {
    try {
      await apiService.resolveCommentThread(commentId)
      await fetchComments()
    } catch (err: any) {
      setError(err.message || 'Failed to resolve thread')
    }
  }

  const handleUnresolveThread = async (commentId: number) => {
    try {
      await apiService.unresolveCommentThread(commentId)
      await fetchComments()
    } catch (err: any) {
      setError(err.message || 'Failed to reopen thread')
    }
  }

  const handleAddReaction = async (commentId: number, emoji: string) => {
    try {
      await apiService.addCommentReaction(commentId, emoji)
      await fetchComments()
      setShowEmojiPicker(null)
    } catch (err: any) {
      setError(err.message || 'Failed to add reaction')
    }
  }

  const handleRemoveReaction = async (commentId: number, emoji: string) => {
    try {
      await apiService.removeCommentReaction(commentId, emoji)
      await fetchComments()
    } catch (err: any) {
      setError(err.message || 'Failed to remove reaction')
    }
  }

  const startNewComment = (line: number) => {
    setNewCommentLine(line)
    setNewCommentContent('')
    setSelectedThread(null)
  }

  const cancelNewComment = () => {
    setNewCommentLine(null)
    setNewCommentContent('')
  }

  const getThreadComments = (threadId: string): CodeComment[] => {
    return comments.filter(c => c.thread_id === threadId)
  }

  // Render a single comment
  const renderComment = (comment: CodeComment, isReply: boolean = false) => {
    const isEditing = editingComment === comment.id
    const isAuthor = comment.author_id === currentUserId
    const reactions = comment.reactions || {}

    return (
      <div
        key={comment.id}
        className={cn(
          'p-3 rounded-lg transition-colors',
          isReply ? 'ml-6 bg-gray-800/30' : 'bg-gray-800/50',
          comment.is_resolved && 'opacity-60'
        )}
      >
        {/* Comment header */}
        <div className="flex items-center justify-between mb-2">
          <div className="flex items-center gap-2">
            <Avatar
              fallback={comment.author_name}
              size="xs"
            />
            <span className="text-sm font-medium text-white">
              {comment.author_name}
            </span>
            <span className="text-xs text-gray-500">
              {new Date(comment.created_at).toLocaleString()}
            </span>
            {comment.is_resolved && !isReply && (
              <Badge variant="success" size="xs">Resolved</Badge>
            )}
          </div>

          {isAuthor && !isEditing && (
            <div className="flex items-center gap-1">
              <Button
                size="xs"
                variant="ghost"
                onClick={() => {
                  setEditingComment(comment.id)
                  setEditContent(comment.content)
                }}
                icon={<Edit2 size={12} />}
                title="Edit"
              />
              <Button
                size="xs"
                variant="ghost"
                onClick={() => handleDeleteComment(comment.id)}
                icon={<Trash2 size={12} />}
                title="Delete"
                className="text-red-400 hover:text-red-300"
              />
            </div>
          )}
        </div>

        {/* Comment content */}
        {isEditing ? (
          <div className="space-y-2">
            <textarea
              value={editContent}
              onChange={(e) => setEditContent(e.target.value)}
              className="w-full bg-gray-700 border border-gray-600 rounded px-3 py-2 text-sm text-white placeholder:text-gray-400 focus:border-cyan-400 focus:outline-none resize-none"
              rows={3}
              autoFocus
            />
            <div className="flex justify-end gap-2">
              <Button
                size="xs"
                variant="ghost"
                onClick={() => {
                  setEditingComment(null)
                  setEditContent('')
                }}
              >
                Cancel
              </Button>
              <Button
                size="xs"
                variant="primary"
                onClick={() => handleUpdateComment(comment.id)}
                disabled={!editContent.trim()}
              >
                Save
              </Button>
            </div>
          </div>
        ) : (
          <p className="text-sm text-gray-300 whitespace-pre-wrap">
            {comment.content}
          </p>
        )}

        {/* Reactions */}
        {!isEditing && Object.keys(reactions).length > 0 && (
          <div className="flex flex-wrap gap-1 mt-2">
            {Object.entries(reactions).map(([emoji, userIds]) => {
              const hasReacted = userIds.includes(currentUserId)
              return (
                <button
                  key={emoji}
                  onClick={() => hasReacted
                    ? handleRemoveReaction(comment.id, emoji)
                    : handleAddReaction(comment.id, emoji)
                  }
                  className={cn(
                    'px-2 py-0.5 rounded-full text-xs flex items-center gap-1 transition-colors',
                    hasReacted
                      ? 'bg-cyan-500/20 border border-cyan-400/50 text-cyan-300'
                      : 'bg-gray-700 border border-gray-600 text-gray-300 hover:border-gray-500'
                  )}
                >
                  <span>{EMOJI_MAP[emoji] || emoji}</span>
                  <span>{userIds.length}</span>
                </button>
              )
            })}
          </div>
        )}

        {/* Action buttons */}
        {!isEditing && !comment.parent_id && (
          <div className="flex items-center gap-2 mt-3 pt-2 border-t border-gray-700/50">
            <Button
              size="xs"
              variant="ghost"
              onClick={() => setReplyingTo(comment.id)}
              icon={<Reply size={12} />}
            >
              Reply
            </Button>

            <div className="relative">
              <Button
                size="xs"
                variant="ghost"
                onClick={() => setShowEmojiPicker(
                  showEmojiPicker === comment.id ? null : comment.id
                )}
                icon={<SmilePlus size={12} />}
              >
                React
              </Button>

              {showEmojiPicker === comment.id && (
                <div className="absolute bottom-full left-0 mb-1 bg-gray-800 border border-gray-700 rounded-lg p-2 flex gap-1 z-10">
                  {EMOJI_REACTIONS.map(emoji => (
                    <button
                      key={emoji}
                      onClick={() => handleAddReaction(comment.id, emoji)}
                      className="p-1 hover:bg-gray-700 rounded transition-colors text-lg"
                      title={emoji.replace('_', ' ')}
                    >
                      {EMOJI_MAP[emoji]}
                    </button>
                  ))}
                </div>
              )}
            </div>

            {!comment.is_resolved ? (
              <Button
                size="xs"
                variant="ghost"
                onClick={() => handleResolveThread(comment.id)}
                icon={<CheckCircle2 size={12} />}
                className="text-green-400 hover:text-green-300"
              >
                Resolve
              </Button>
            ) : (
              <Button
                size="xs"
                variant="ghost"
                onClick={() => handleUnresolveThread(comment.id)}
                icon={<RotateCcw size={12} />}
              >
                Reopen
              </Button>
            )}
          </div>
        )}

        {/* Reply input */}
        {replyingTo === comment.id && (
          <div className="mt-3 pt-3 border-t border-gray-700/50 space-y-2">
            <textarea
              value={replyContent}
              onChange={(e) => setReplyContent(e.target.value)}
              placeholder="Write a reply..."
              className="w-full bg-gray-700 border border-gray-600 rounded px-3 py-2 text-sm text-white placeholder:text-gray-400 focus:border-cyan-400 focus:outline-none resize-none"
              rows={2}
              autoFocus
            />
            <div className="flex justify-end gap-2">
              <Button
                size="xs"
                variant="ghost"
                onClick={() => {
                  setReplyingTo(null)
                  setReplyContent('')
                }}
              >
                Cancel
              </Button>
              <Button
                size="xs"
                variant="primary"
                onClick={() => handleReply(comment.id, comment.thread_id)}
                disabled={!replyContent.trim() || isCreating}
                loading={isCreating}
                icon={<Send size={12} />}
              >
                Reply
              </Button>
            </div>
          </div>
        )}

        {/* Render replies */}
        {comment.replies && comment.replies.length > 0 && (
          <div className="mt-3 space-y-2">
            {comment.replies.map(reply => renderComment(reply, true))}
          </div>
        )}
      </div>
    )
  }

  // Render thread panel
  const renderThreadPanel = () => {
    if (!selectedThread) return null

    const threadComments = getThreadComments(selectedThread)
    if (threadComments.length === 0) return null

    const rootComment = threadComments.find(c => !c.parent_id) || threadComments[0]
    const replies = threadComments.filter(c => c.parent_id)

    return (
      <div className="border-t border-gray-700 p-4">
        <div className="flex items-center justify-between mb-4">
          <h4 className="text-sm font-medium text-white">
            Thread on line {rootComment.start_line}
          </h4>
          <Button
            size="xs"
            variant="ghost"
            onClick={() => setSelectedThread(null)}
            icon={<X size={14} />}
          />
        </div>

        <div className="space-y-3 max-h-96 overflow-y-auto">
          {renderComment(rootComment)}
          {replies.map(reply => renderComment(reply, true))}
        </div>
      </div>
    )
  }

  return (
    <Card
      variant="cyberpunk"
      padding="none"
      className={cn('flex flex-col h-full', className)}
    >
      {/* Header */}
      <div className="p-4 border-b border-gray-700/50">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <MessageSquare className="w-4 h-4 text-cyan-400" />
            <h3 className="text-sm font-semibold text-white">Comments</h3>
            <Badge variant="outline" size="xs">
              {markers.filter(m => !m.isResolved).length}
            </Badge>
          </div>

          <label className="flex items-center gap-2 text-xs text-gray-400 cursor-pointer">
            <input
              type="checkbox"
              checked={showResolved}
              onChange={(e) => setShowResolved(e.target.checked)}
              className="rounded border-gray-600 bg-gray-800 text-cyan-500 focus:ring-cyan-500"
            />
            Show resolved
          </label>
        </div>
      </div>

      {/* Loading state */}
      {loading && (
        <div className="flex-1 flex items-center justify-center p-4">
          <Loading size="md" variant="spinner" />
        </div>
      )}

      {/* Error state */}
      {error && (
        <div className="p-4">
          <div className="bg-red-500/10 border border-red-500/30 rounded-lg p-3 flex items-start gap-2">
            <AlertCircle className="w-4 h-4 text-red-400 mt-0.5" />
            <div>
              <p className="text-sm text-red-400">{error}</p>
              <Button
                size="xs"
                variant="ghost"
                onClick={fetchComments}
                className="mt-2 text-red-300"
              >
                Try again
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Comment markers / thread list */}
      {!loading && !error && (
        <div className="flex-1 overflow-y-auto">
          {markers.length === 0 ? (
            <div className="p-4 text-center text-gray-500">
              <MessageSquare className="w-8 h-8 mx-auto mb-2 opacity-50" />
              <p className="text-sm">No comments yet</p>
              <p className="text-xs mt-1">Click on a line number to add a comment</p>
            </div>
          ) : (
            <div className="p-2 space-y-1">
              {markers.map(marker => (
                <button
                  key={marker.threadId}
                  onClick={() => {
                    setSelectedThread(
                      selectedThread === marker.threadId ? null : marker.threadId
                    )
                    onCommentClick?.(marker.lineNumber)
                  }}
                  className={cn(
                    'w-full p-3 rounded-lg text-left transition-colors',
                    selectedThread === marker.threadId
                      ? 'bg-cyan-500/10 border border-cyan-500/30'
                      : 'bg-gray-800/30 hover:bg-gray-800/50 border border-transparent',
                    marker.isResolved && 'opacity-60'
                  )}
                >
                  <div className="flex items-center justify-between">
                    <span className="text-sm text-white">
                      Line {marker.lineNumber}
                    </span>
                    <div className="flex items-center gap-2">
                      <Badge
                        variant={marker.isResolved ? 'outline' : 'secondary'}
                        size="xs"
                      >
                        {marker.commentCount} {marker.commentCount === 1 ? 'comment' : 'comments'}
                      </Badge>
                      {marker.isResolved && (
                        <Check className="w-3 h-3 text-green-400" />
                      )}
                      {selectedThread === marker.threadId ? (
                        <ChevronUp className="w-3 h-3 text-gray-400" />
                      ) : (
                        <ChevronDown className="w-3 h-3 text-gray-400" />
                      )}
                    </div>
                  </div>
                </button>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Thread detail panel */}
      {renderThreadPanel()}

      {/* New comment form */}
      {newCommentLine !== null && (
        <div className="border-t border-gray-700 p-4">
          <div className="flex items-center justify-between mb-3">
            <h4 className="text-sm font-medium text-white">
              New comment on line {newCommentLine}
            </h4>
            <Button
              size="xs"
              variant="ghost"
              onClick={cancelNewComment}
              icon={<X size={14} />}
            />
          </div>

          <textarea
            value={newCommentContent}
            onChange={(e) => setNewCommentContent(e.target.value)}
            placeholder="Write a comment..."
            className="w-full bg-gray-800 border border-gray-600 rounded-lg px-3 py-2 text-sm text-white placeholder:text-gray-400 focus:border-cyan-400 focus:outline-none resize-none mb-3"
            rows={3}
            autoFocus
          />

          <div className="flex justify-end gap-2">
            <Button
              size="sm"
              variant="ghost"
              onClick={cancelNewComment}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              variant="primary"
              onClick={handleCreateComment}
              disabled={!newCommentContent.trim() || isCreating}
              loading={isCreating}
              icon={<Send size={14} />}
            >
              Comment
            </Button>
          </div>
        </div>
      )}
    </Card>
  )
}

// Export component for getting markers (used by Monaco editor gutter)
export const useCodeCommentMarkers = (fileId: number | undefined) => {
  const [markers, setMarkers] = useState<CommentMarker[]>([])

  useEffect(() => {
    if (!fileId) {
      setMarkers([])
      return
    }

    const fetchMarkers = async () => {
      try {
        const response = await apiService.getFileComments(fileId, {
          include_resolved: false,
        })

        const threadMap = new Map<string, CodeComment[]>()
        response.comments.forEach(comment => {
          const existing = threadMap.get(comment.thread_id) || []
          threadMap.set(comment.thread_id, [...existing, comment])
        })

        const newMarkers: CommentMarker[] = []
        threadMap.forEach((threadComments, threadId) => {
          const rootComment = threadComments.find(c => !c.parent_id) || threadComments[0]
          newMarkers.push({
            lineNumber: rootComment.start_line,
            threadId,
            commentCount: threadComments.length + (threadComments[0]?.replies?.length || 0),
            isResolved: rootComment.is_resolved,
          })
        })

        setMarkers(newMarkers.sort((a, b) => a.lineNumber - b.lineNumber))
      } catch (err) {
        console.error('Failed to fetch comment markers:', err)
      }
    }

    fetchMarkers()
  }, [fileId])

  return markers
}

export default CodeComments
