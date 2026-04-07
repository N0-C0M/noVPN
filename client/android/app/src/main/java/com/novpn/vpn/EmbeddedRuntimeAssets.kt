package com.novpn.vpn

import android.content.Context
import android.os.Build
import java.io.IOException

object EmbeddedRuntimeAssets {
    fun resolveBinaryAssetPath(context: Context, binaryName: String): String {
        return resolveBinaryAssetPathOrNull(context, binaryName)
            ?: throw IllegalStateException(
                "Embedded binary asset missing for $binaryName. " +
                    "Expected one of: ${buildAbiAssetCandidates(binaryName).joinToString()}."
            )
    }

    fun resolveBinaryAssetPathOrNull(context: Context, binaryName: String): String? {
        return buildAbiAssetCandidates(binaryName).firstOrNull { assetExists(context, it) }
    }

    fun assetExists(context: Context, assetPath: String): Boolean {
        return try {
            context.assets.open(assetPath).close()
            true
        } catch (_: IOException) {
            false
        }
    }

    fun assetLabel(assetPath: String): String {
        val segments = assetPath.split('/')
        return when {
            segments.size >= 3 -> segments[1]
            else -> assetPath
        }
    }

    private fun buildAbiAssetCandidates(binaryName: String): List<String> {
        val supported = Build.SUPPORTED_ABIS
            .flatMap(::abiAliases)
            .distinct()

        val candidates = supported.map { abi -> "bin/$abi/$binaryName" }.toMutableList()
        candidates += "bin/$binaryName"
        return candidates
    }

    private fun abiAliases(abi: String): List<String> {
        val aliases = mutableListOf(abi)
        when (abi) {
            "arm64-v8a" -> aliases += listOf("arm64", "aarch64")
            "armeabi-v7a" -> aliases += listOf("armeabi", "armv7")
            "x86_64" -> aliases += listOf("amd64")
            "x86" -> aliases += listOf("i686", "i386")
        }
        return aliases
    }
}
