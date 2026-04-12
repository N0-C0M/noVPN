package com.novpn.data

import android.content.Context

class ClientPreferences(context: Context) {
    private val preferences =
        context.getSharedPreferences(PREFERENCE_FILE, Context.MODE_PRIVATE)

    fun isBypassRuEnabled(): Boolean {
        return preferences.getBoolean(KEY_BYPASS_RU, true)
    }

    fun excludedPackages(): List<String> {
        val hasStoredSelection = preferences.contains(KEY_SELECTED_PACKAGES) || preferences.contains(KEY_EXCLUDED_PACKAGES)
        if (!hasStoredSelection && isDefaultWhitelistEnabled()) {
            return defaultWhitelistPackages()
        }

        return preferences.getStringSet(
            KEY_SELECTED_PACKAGES,
            preferences.getStringSet(KEY_EXCLUDED_PACKAGES, emptySet())
        )
            .orEmpty()
            .sorted()
    }

    fun appRoutingMode(): AppRoutingMode {
        if (!preferences.contains(KEY_APP_ROUTING_MODE)) {
            return if (isDefaultWhitelistEnabled()) {
                AppRoutingMode.ONLY_SELECTED
            } else {
                AppRoutingMode.EXCLUDE_SELECTED
            }
        }
        return AppRoutingMode.fromStorage(
            preferences.getString(KEY_APP_ROUTING_MODE, AppRoutingMode.ONLY_SELECTED.storageValue)
        )
    }

    fun isDefaultWhitelistEnabled(): Boolean {
        return preferences.getBoolean(KEY_DEFAULT_WHITELIST_ENABLED, true)
    }

    fun forceServerIpMode(): Boolean {
        return preferences.getBoolean(KEY_FORCE_SERVER_IP_MODE, true)
    }

    fun trafficObfuscationStrategy(): TrafficObfuscationStrategy {
        return TrafficObfuscationStrategy.fromStorage(
            preferences.getString(KEY_TRAFFIC_STRATEGY, TrafficObfuscationStrategy.BALANCED.storageValue)
        )
    }

    fun patternMaskingStrategy(): PatternMaskingStrategy {
        return PatternMaskingStrategy.fromStorage(
            preferences.getString(KEY_PATTERN_STRATEGY, PatternMaskingStrategy.STEADY.storageValue)
        )
    }

    fun selectedProfileId(defaultProfileId: String): String {
        return preferences.getString(KEY_SELECTED_PROFILE, defaultProfileId) ?: defaultProfileId
    }

    fun inviteCode(): String {
        return preferences.getString(KEY_INVITE_CODE, "").orEmpty()
    }

    fun isInitialRuAppAuditPending(): Boolean {
        return preferences.getBoolean(KEY_INITIAL_RU_APP_AUDIT_PENDING, true)
    }

    fun knownInstalledPackages(): Set<String> {
        return preferences.getStringSet(KEY_KNOWN_INSTALLED_PACKAGES, emptySet())
            .orEmpty()
            .map { it.trim() }
            .filter { it.isNotBlank() }
            .toSet()
    }

    fun disguiseIdentity(): DisguiseIdentity {
        val defaultIdentity = DisguiseIdentityGenerator.defaultIdentity()
        return DisguiseIdentity(
            appName = preferences.getString(KEY_DISGUISE_APP_NAME, defaultIdentity.appName).orEmpty().ifBlank {
                defaultIdentity.appName
            },
            applicationId = preferences.getString(KEY_DISGUISE_APP_ID, defaultIdentity.applicationId).orEmpty().ifBlank {
                defaultIdentity.applicationId
            },
            rebuildCommand = preferences.getString(KEY_DISGUISE_COMMAND, defaultIdentity.rebuildCommand).orEmpty().ifBlank {
                defaultIdentity.rebuildCommand
            }
        )
    }

    fun saveBypassRu(enabled: Boolean) {
        preferences.edit().putBoolean(KEY_BYPASS_RU, enabled).apply()
    }

    fun saveExcludedPackages(packageNames: List<String>) {
        preferences.edit()
            .putStringSet(KEY_SELECTED_PACKAGES, packageNames.distinct().toSet())
            .putStringSet(KEY_EXCLUDED_PACKAGES, packageNames.distinct().toSet())
            .apply()
    }

    fun saveAppRoutingMode(mode: AppRoutingMode) {
        preferences.edit().putString(KEY_APP_ROUTING_MODE, mode.storageValue).apply()
    }

    fun saveDefaultWhitelistEnabled(enabled: Boolean) {
        preferences.edit().putBoolean(KEY_DEFAULT_WHITELIST_ENABLED, enabled).apply()
    }

    fun saveForceServerIpMode(enabled: Boolean) {
        preferences.edit().putBoolean(KEY_FORCE_SERVER_IP_MODE, enabled).apply()
    }

    fun saveTrafficObfuscationStrategy(strategy: TrafficObfuscationStrategy) {
        preferences.edit().putString(KEY_TRAFFIC_STRATEGY, strategy.storageValue).apply()
    }

    fun savePatternMaskingStrategy(strategy: PatternMaskingStrategy) {
        preferences.edit().putString(KEY_PATTERN_STRATEGY, strategy.storageValue).apply()
    }

    fun saveSelectedProfileId(profileId: String) {
        preferences.edit().putString(KEY_SELECTED_PROFILE, profileId).apply()
    }

    fun saveInviteCode(code: String) {
        preferences.edit().putString(KEY_INVITE_CODE, code.trim()).apply()
    }

    fun markInitialRuAppAuditCompleted() {
        preferences.edit().putBoolean(KEY_INITIAL_RU_APP_AUDIT_PENDING, false).apply()
    }

    fun saveKnownInstalledPackages(packageNames: Collection<String>) {
        preferences.edit()
            .putStringSet(KEY_KNOWN_INSTALLED_PACKAGES, packageNames.map { it.trim() }.filter { it.isNotBlank() }.toSet())
            .apply()
    }

    fun saveDisguiseIdentity(identity: DisguiseIdentity) {
        preferences.edit()
            .putString(KEY_DISGUISE_APP_NAME, identity.appName)
            .putString(KEY_DISGUISE_APP_ID, identity.applicationId)
            .putString(KEY_DISGUISE_COMMAND, identity.rebuildCommand)
            .apply()
    }

    companion object {
        private const val PREFERENCE_FILE = "novpn_client_preferences"
        private const val KEY_BYPASS_RU = "bypass_ru"
        private const val KEY_APP_ROUTING_MODE = "app_routing_mode"
        private const val KEY_DEFAULT_WHITELIST_ENABLED = "default_whitelist_enabled"
        private const val KEY_FORCE_SERVER_IP_MODE = "force_server_ip_mode"
        private const val KEY_SELECTED_PACKAGES = "selected_packages"
        private const val KEY_EXCLUDED_PACKAGES = "excluded_packages"
        private const val KEY_TRAFFIC_STRATEGY = "traffic_strategy"
        private const val KEY_PATTERN_STRATEGY = "pattern_strategy"
        private const val KEY_SELECTED_PROFILE = "selected_profile"
        private const val KEY_INVITE_CODE = "invite_code"
        private const val KEY_INITIAL_RU_APP_AUDIT_PENDING = "initial_ru_app_audit_pending"
        private const val KEY_KNOWN_INSTALLED_PACKAGES = "known_installed_packages"
        private const val KEY_DISGUISE_APP_NAME = "disguise_app_name"
        private const val KEY_DISGUISE_APP_ID = "disguise_app_id"
        private const val KEY_DISGUISE_COMMAND = "disguise_command"

        private val DEFAULT_WHITELIST_PACKAGES = listOf(
            "com.google.android.youtube",
            "com.google.android.apps.youtube.music",
            "com.google.android.apps.youtube.kids",
            "com.supercell.brawlstars",
            "org.telegram.messenger",
            "org.telegram.plus",
            "com.instagram.android",
            "com.twitter.android",
            "com.supercell.clashroyale",
            "com.supercell.clashofclans",
            "mega.privacy.android.app",
            "com.openai.chatgpt",
            "com.google.android.apps.bard"
        ).sorted()
    }

    fun defaultWhitelistPackages(): List<String> {
        return DEFAULT_WHITELIST_PACKAGES
    }
}
