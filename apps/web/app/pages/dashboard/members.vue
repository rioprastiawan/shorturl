<script setup lang="ts">
import type { Member, Role } from '~/types/api'
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
const inviteRole = ref<Role>('member')
const inviting = ref(false)
const inviteEmailError = ref<string | undefined>()
const inviteError = ref<string | null>(null)

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
      // There are no invitations in the MVP: the server can only attach an
      // account that already exists, so say so plainly instead of showing a
      // bare "not found".
      if (e.status === 404) {
        inviteEmailError.value = 'No user with that email address. Ask them to register first.'
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

function onRoleSelect(member: Member, event: Event) {
  const value = (event.target as HTMLSelectElement).value as Role
  changeRole(member, value)
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
      <h1 class="text-xl font-semibold tracking-tight">
        Members
      </h1>
      <p class="text-sm text-(--color-content-muted)">
        Who can see and change things in {{ ws.active.value?.name ?? 'this workspace' }}.
      </p>
    </header>

    <UiCard
      v-if="canManage"
      title="Add a member"
      description="The person needs a ShortURL account already — there are no email invitations yet."
    >
      <form class="flex flex-col gap-4" novalidate @submit.prevent="addMember">
        <DashboardFormAlert v-if="inviteError">
          {{ inviteError }}
        </DashboardFormAlert>

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

          <div class="flex flex-col gap-1.5 sm:w-40">
            <label for="invite-role" class="text-sm font-medium">Role</label>
            <select
              id="invite-role"
              v-model="inviteRole"
              class="rounded-md border border-(--color-border-strong) bg-transparent px-3 py-2 text-sm"
            >
              <option value="member">
                Member
              </option>
              <option value="admin">
                Admin
              </option>
            </select>
          </div>

          <div class="sm:pt-7">
            <UiButton type="submit" :loading="inviting">
              Add
            </UiButton>
          </div>
        </div>
      </form>
    </UiCard>

    <UiCard :title="`Members (${members.length})`" :padded="false">
      <p v-if="pending && !data" class="px-5 py-6 text-sm text-(--color-content-muted)" role="status">
        Loading…
      </p>

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
            <select
              v-if="canManage"
              :id="`role-${member.user_id}`"
              :value="member.role"
              :disabled="savingRoleFor === member.user_id"
              class="rounded-md border border-(--color-border-strong) bg-transparent px-2 py-1 text-sm disabled:opacity-50"
              @change="onRoleSelect(member, $event)"
            >
              <option value="member">
                Member
              </option>
              <option value="admin">
                Admin
              </option>
            </select>
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
