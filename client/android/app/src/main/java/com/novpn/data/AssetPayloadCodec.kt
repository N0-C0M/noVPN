package com.novpn.data

import android.content.Context
import java.io.ByteArrayInputStream
import java.security.MessageDigest
import java.util.Base64
import java.util.zip.GZIPInputStream

object AssetPayloadCodec {
    fun decodeAssetText(context: Context, assetPath: String, salt: String): String {
        val encodedPayload = context.assets.open(assetPath)
            .bufferedReader()
            .use { it.readText() }
        return decodeText(encodedPayload, salt)
    }

    fun decodeText(encodedPayload: String, salt: String): String {
        val obfuscated = Base64.getDecoder().decode(encodedPayload.trim())
        val key = deriveKey(salt)
        val compressed = xorWithKey(obfuscated, key)
        val plain = GZIPInputStream(ByteArrayInputStream(compressed)).use { it.readBytes() }
        return plain.toString(Charsets.UTF_8)
    }

    private fun deriveKey(salt: String): ByteArray {
        return MessageDigest.getInstance("SHA-256")
            .digest(salt.toByteArray(Charsets.UTF_8))
    }

    private fun xorWithKey(input: ByteArray, key: ByteArray): ByteArray {
        return ByteArray(input.size) { index ->
            (input[index].toInt() xor key[index % key.size].toInt()).toByte()
        }
    }
}
