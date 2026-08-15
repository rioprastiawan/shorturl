<script setup lang="ts">
import type { Member, Role, WorkspaceInvitation } from '~/types/api'
import { ApiError } from '~/composables/useApi'
import { formatDate } from '~/components/dashboard/format'
import { useServices } from '~/services'

definePageMeta({ middleware: 'auth' })

useHead({ title: 'Members · ShortURL' })

const ws = useWorkspaces()
const session = useSession()
const { workspaces } = useServices()
const toast = useToast()

const { data, pending, error, refresh } = await useAsyncData(
  'workspace-members',
  () => workspaces.members(ws.requireActiveId()),
  { watch: [ws.activeId] },
)

const members = computed(() => data.value?.data ?? [])
const canManage = computed(() => ws.role.value === 'owner' || ws.role.value === 'admin')
const currentUserId = computed(() => session.user.value?.id ?? null)

const loadError = computed(() => {
  const e = error.value
  if (!e) return null
  return e instanceof ApiError ? e.message : 'Could not load members.'
})

// Owner is called out because it is the one role that cannot be changed here;
// admin and member are both editable and read as ordinary states.
function roleTone(role: Role): 'info' | 'neutral' {
  return role === 'owner' ? 'info' : 'neutral'
}

function isSelf(member: Member): boolean {
  return member.user_id === currentUserId.value
}

/* ------------------------------------------------------------ add member */

const inviteEmail = ref('')
const inviteRole = ref<Exclude<Role, 'owner'>>('member')
const roleOptions = [
  { value: 'member', label: 'Member' },
  { value: 'admin', label: 'Admin' },
]
const inviting = ref(false)
const inviteEmailError = ref<string | undefined>()
const inviteError = ref<string | null>(null)
const invitationLink = ref('')
const generatingInvitation = ref(false)
const invitations = ref<WorkspaceInvitation[]>([])
const invitationsLoading = ref(false)
const invitationActionId = ref<string>()

async function loadInvitations() {
  if (!canManage.value || !ws.activeId.value) return
  invitationsLoading.value = true
  try {
    invitations.value = (await workspaces.invitations(ws.requireActiveId())).data
  } catch (e) {
    inviteError.value = e instanceof ApiError ? e.message : 'Could not load invitations.'
  } finally {
    invitationsLoading.value = false
  }
}

watch([ws.activeId, canManage], loadInvitations, { immediate: true })

function invitationState(invitation: WorkspaceInvitation) {
  if (invitation.accepted_at) return { label: 'Used', tone: 'neutral' as const }
  if (invitation.revoked_at) return { label: 'Revoked', tone: 'neutral' as const }
  if (new Date(invitation.expires_at).getTime() <= Date.now()) return { label: 'Expired', tone: 'warning' as const }
  return { label: 'Active', tone: 'success' as const }
}

function revealInvitation(invitation: WorkspaceInvitation) {
  invitationLink.value = `${window.location.origin}/register?invite=${encodeURIComponent(invitation.token)}`
}

async function createInvitation() {
  generatingInvitation.value = true
  inviteError.value = null
  try {
    const invitation = await workspaces.createInvitation(ws.requireActiveId(), { role: inviteRole.value })
    revealInvitation(invitation)
    await loadInvitations()
  } catch (e) {
    inviteError.value = e instanceof ApiError ? e.message : 'Could not create the invitation.'
  } finally {
    generatingInvitation.value = false
  }
}

async function renewInvitation(invitation: WorkspaceInvitation) {
  invitationActionId.value = invitation.id
  try {
    const renewed = await workspaces.renewInvitation(ws.requireActiveId(), invitation.id)
    revealInvitation(renewed)
    await loadInvitations()
    toast.success('A new invitation link was generated.')
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : 'Could not renew the invitation.')
  } finally {
    invitationActionId.value = undefined
  }
}

async function revokeInvitation(invitation: WorkspaceInvitation) {
  invitationActionId.value = invitation.id
  try {
    await workspaces.revokeInvitation(ws.requireActiveId(), invitation.id)
    await loadInvitations()
    toast.success('Invitation revoked.')
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : 'Could not revoke the invitation.')
  } finally {
    invitationActionId.value = undefined
  }
}

async function addMember() {
  inviteEmailError.value = undefined
  inviteError.value = null
  inviting.value = true

  try {
    await workspaces.addMember(ws.requireActiveId(), {
      email: inviteEmail.value.trim(),
      role: inviteRole.value,
    })
    inviteEmail.value = ''
    inviteRole.value = 'member'
    await refresh()
    toast.success('Member added')
  } catch (e) {
    if (e instanceof ApiError) {
      if (e.status === 404) {
        await createInvitation()
        if (invitationLink.value) {
          toast.success('User not found, so an invitation link was generated instead.')
        }
      } else {
        inviteEmailError.value = e.field('email')
        if (!inviteEmailError.value) inviteError.value = e.message
      }
    } else {
      inviteError.value = 'Could not add the member.'
    }
  } finally {
    inviting.value = false
  }
}

/* ----------------------------------------------------------- change role */

const savingRoleFor = ref<string | null>(null)

async function changeRole(member: Member, next: Role) {
  if (next === member.role) return

  savingRoleFor.value = member.user_id
  try {
    await workspaces.updateMemberRole(ws.requireActiveId(), member.user_id, { role: next })
    await refresh()
    toast.success(`${member.name} is now ${next === 'admin' ? 'an admin' : 'a member'}`)
  } catch (e) {
    // Put the select back where it was; the list is the source of truth.
    await refresh()
    toast.error(e instanceof ApiError ? e.message : 'Could not change the role')
  } finally {
    savingRoleFor.value = null
  }
}

/* ---------------------------------------------------------------- remove */

const removeTarget = ref<Member | null>(null)
const removing = ref(false)
const removeError = ref<string | null>(null)

const leavingSelf = computed(() => !!removeTarget.value && isSelf(removeTarget.value))

const removeOpen = computed({
  get: () => removeTarget.value !== null,
  set: (open: boolean) => {
    if (!open) {
      removeTarget.value = null
      removeError.value = null
    }
  },
})

function askRemove(member: Member) {
  removeError.value = null
  removeTarget.value = member
}

async function confirmRemove() {
  const member = removeTarget.value
  if (!member) return

  const self = isSelf(member)
  removing.value = true
  removeError.value = null

  try {
    await workspaces.removeMember(ws.requireActiveId(), member.user_id)

    if (self) {
      // We are no longer a member, so this list would 403 on the next fetch.
      // Reload the workspace list and let it pick a new active workspace.
      removeTarget.value = null
      await ws.load(true)
      toast.success('You left the workspace')
      await navigateTo('/dashboard')
      return
    }

    removeTarget.value = null
    await refresh()
    toast.success(`${member.name} was removed`)
  } catch (e) {
    removeError.value = e instanceof ApiError ? e.message : 'Could not remove the member.'
  } finally {
    removing.value = false
  }
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <header>
      <p class="mb-1 text-sm font-semibold text-(--color-accent)">Manage</p>
      <h1 class="text-2xl font-bold tracking-tight sm:text-3xl">
        Team members
      </h1>
      <p class="text-sm text-(--color-content-muted)">
        Invite people and decide what they can manage in {{ ws.active.value?.name ?? 'this workspace' }}.
      </p>
    </header>

    <UiCard
      v-if="canManage"
      title="Add a member"
      description="Add an existing account by email, or generate a one-time registration link for someone new."
    >
      <form class="flex flex-col gap-4" novalidate @submit.prevent="addMember">
        <DashboardFormAlert v-if="inviteError">
          {{ inviteError }}
        </DashboardFormAlert>

        <div v-if="invitationLink" class="rounded-md border border-(--color-border) bg-(--color-surface-muted) p-3">
          <p class="mb-2 text-sm font-medium">Invitation link</p>
          <UiCopyButton :value="invitationLink" show-value label="Copy invitation link" />
          <p class="mt-2 text-xs text-(--color-content-muted)">This link can be used once and expires after 7 days.</p>
        </div>

        <div class="flex flex-col gap-4 sm:flex-row sm:items-start">
          <div class="flex-1">
            <UiInput
              v-model="inviteEmail"
              label="Email"
              type="email"
              placeholder="teammate@example.com"
              autocomplete="off"
              required
              :error="inviteEmailError"
            />
          </div>

          <div class="sm:w-40">
            <UiSelect v-model="inviteRole" input-id="invite-role" label="Role" :options="roleOptions" />
          </div>

          <div class="sm:pt-7">
            <UiButton type="submit" :loading="inviting">
              Add existing user
            </UiButton>
          </div>
        </div>

        <div class="flex items-center gap-3 border-t border-(--color-border) pt-4">
          <UiButton type="button" variant="secondary" :loading="generatingInvitation" @click="createInvitation">
            Generate invitation link
          </UiButton>
          <span class="text-xs text-(--color-content-muted)">Uses the selected role; no email is required.</span>
        </div>
      </form>
    </UiCard>

    <UiCard v-if="canManage" :title="`Invitations (${invitations.length})`" :padded="false">
      <div v-if="invitationsLoading" class="space-y-4 px-5 py-5" role="status" aria-label="Loading invitations">
        <div v-for="row in 3" :key="row" class="flex items-center justify-between gap-4"><div class="flex-1 space-y-2"><UiSkeleton width="40%" /><UiSkeleton height="0.65rem" width="65%" /></div><UiSkeleton height="2rem" width="5rem" rounded="lg" /></div>
      </div>
      <UiEmptyState
        v-else-if="!invitations.length"
        title="No invitations"
        description="Generated invitation links will appear here."
      />
      <ul v-else class="divide-y divide-(--color-border)">
        <li
          v-for="invitation in invitations"
          :key="invitation.id"
          class="flex flex-wrap items-center gap-3 px-5 py-3"
        >
          <div class="min-w-0 flex-1">
            <p class="text-sm font-medium capitalize">
              {{ invitation.role }} invitation
            </p>
            <p class="text-xs text-(--color-content-muted)">
              Created {{ formatDate(invitation.created_at) }} · Expires {{ formatDate(invitation.expires_at) }}
            </p>
          </div>
          <UiBadge :tone="invitationState(invitation).tone" dot>
            {{ invitationState(invitation).label }}
          </UiBadge>
          <UiButton
            v-if="!invitation.accepted_at"
            variant="secondary"
            size="sm"
            :loading="invitationActionId === invitation.id"
            @click="renewInvitation(invitation)"
          >
            Generate new link
          </UiButton>
          <UiButton
            v-if="!invitation.accepted_at && !invitation.revoked_at && invitationState(invitation).label === 'Active'"
            variant="ghost"
            size="sm"
            :disabled="invitationActionId === invitation.id"
            @click="revokeInvitation(invitation)"
          >
            <span class="text-(--color-danger)">Revoke</span>
          </UiButton>
        </li>
      </ul>
    </UiCard>

    <UiCard :title="`Members (${members.length})`" :padded="false">
      <div v-if="pending && !data" role="status" aria-label="Loading members">
        <UiSkeletonTable :rows="5" :columns="4" />
      </div>

      <div v-else-if="loadError" class="flex flex-col items-start gap-3 px-5 py-6">
        <p class="text-sm text-(--color-danger)" role="alert">
          {{ loadError }}
        </p>
        <UiButton variant="secondary" size="sm" @click="refresh()">
          Retry
        </UiButton>
      </div>

      <UiEmptyState
        v-else-if="!members.length"
        title="No members"
        description="This workspace has no members yet."
      />

      <ul v-else class="divide-y divide-(--color-border)">
        <li
          v-for="member in members"
          :key="member.user_id"
          class="flex flex-wrap items-center gap-x-4 gap-y-3 px-5 py-3"
        >
          <div class="min-w-0 flex-1">
            <p class="truncate text-sm font-medium">
              {{ member.name }}
              <span v-if="isSelf(member)" class="font-normal text-(--color-content-subtle)">(you)</span>
            </p>
            <p class="truncate text-xs text-(--color-content-muted)">
              {{ member.email }}
            </p>
            <p class="mt-0.5 text-xs text-(--color-content-subtle)">
              Joined {{ formatDate(member.created_at) }}
            </p>
          </div>

          <!-- The owner is fixed: ownership transfer does not exist yet, so an
               editable control here could only ever produce a 409. -->
          <UiBadge v-if="member.role === 'owner'" tone="info">
            Owner
          </UiBadge>

          <template v-else>
            <label v-if="canManage" class="sr-only" :for="`role-${member.user_id}`">
              Role for {{ member.name }}
            </label>
            <UiSelect
              v-if="canManage"
              :input-id="`role-${member.user_id}`"
              :model-value="member.role"
              :options="roleOptions"
              size="sm"
              :disabled="savingRoleFor === member.user_id"
              class="w-32"
              @update:model-value="value => changeRole(member, value as Role)"
            />
            <UiBadge v-else :tone="roleTone(member.role)" class="capitalize">
              {{ member.role }}
            </UiBadge>
          </template>

          <div class="flex shrink-0 items-center gap-2">
            <UiButton
              v-if="member.role !== 'owner' && isSelf(member)"
              variant="secondary"
              size="sm"
              @click="askRemove(member)"
            >
              Leave workspace
            </UiButton>
            <UiButton
              v-else-if="member.role !== 'owner' && canManage"
              variant="ghost"
              size="sm"
              @click="askRemove(member)"
            >
              Remove
            </UiButton>
          </div>
        </li>
      </ul>
    </UiCard>

    <UiModal
      v-model="removeOpen"
      :title="leavingSelf ? 'Leave this workspace?' : 'Remove this member?'"
      :description="leavingSelf
        ? 'You will lose access to its links, domains, and analytics. An admin can add you back.'
        : 'They lose access immediately. Links they created stay in the workspace.'"
      danger
    >
      <div class="flex flex-col gap-3">
        <DashboardFormAlert v-if="removeError">
          {{ removeError }}
        </DashboardFormAlert>
        <p v-if="removeTarget" class="text-sm">
          <span class="font-medium">{{ removeTarget.name }}</span>
          <span class="text-(--color-content-muted)"> · {{ removeTarget.email }}</span>
        </p>
      </div>

      <template #actions>
        <UiButton variant="secondary" :disabled="removing" @click="removeOpen = false">
          Cancel
        </UiButton>
        <UiButton variant="danger" :loading="removing" @click="confirmRemove">
          {{ leavingSelf ? 'Leave workspace' : 'Remove member' }}
        </UiButton>
      </template>
    </UiModal>
  </div>
</template>
