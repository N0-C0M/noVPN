package com.novpn.data

import java.util.Locale
import kotlin.random.Random

data class DisguiseIdentity(
    val appName: String,
    val applicationId: String,
    val rebuildCommand: String
)

object DisguiseIdentityGenerator {
    private val adjectives = listOf(
        "silent",
        "calm",
        "misty",
        "hidden",
        "ember",
        "frost",
        "cinder",
        "silver"
    )

    private val animals = listOf(
        "turtle",
        "sparrow",
        "otter",
        "badger",
        "falcon",
        "seal",
        "fox",
        "rook"
    )

    fun defaultIdentity(): DisguiseIdentity {
        return buildIdentity(appName = "Safety Turtle", applicationId = "safety.turtle")
    }

    fun randomIdentity(): DisguiseIdentity {
        val adjective = adjectives.random()
        val animal = animals.random()
        val appName = adjective.replaceFirstChar { it.titlecase(Locale.ROOT) } +
            " " +
            animal.replaceFirstChar { it.titlecase(Locale.ROOT) }
        val suffix = Random.nextInt(11, 99)
        val applicationId = "safety.${adjective.lowercase(Locale.ROOT)}.${animal.lowercase(Locale.ROOT)}$suffix"
        return buildIdentity(appName, applicationId)
    }

    private fun buildIdentity(appName: String, applicationId: String): DisguiseIdentity {
        val command = ".\\gradlew.bat :app:assembleDebug -PnovpnAppName=\"$appName\" -PnovpnAppId=$applicationId"
        return DisguiseIdentity(
            appName = appName,
            applicationId = applicationId,
            rebuildCommand = command
        )
    }
}
