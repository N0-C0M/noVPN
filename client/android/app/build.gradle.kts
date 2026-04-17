import org.gradle.api.file.RelativePath
import org.gradle.api.tasks.Sync
import java.io.ByteArrayOutputStream
import java.io.File
import java.security.MessageDigest
import java.util.Base64
import java.util.zip.GZIPOutputStream

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

layout.buildDirectory.set(file("build-codex"))

val disguiseAppId = providers.gradleProperty("novpnAppId").orNull
    ?: System.getenv("NOVPN_APP_ID")
    ?: "safety.turtle"
val disguiseAppName = providers.gradleProperty("novpnAppName").orNull
    ?: System.getenv("NOVPN_APP_NAME")
    ?: "Safety Turtle"

val embeddedRuntimeExecLibsDir = layout.buildDirectory.dir("generated/embeddedRuntimeExecJniLibs")
val securedAssetsDir = layout.buildDirectory.dir("generated/securedAssets")
val repoRootDir = rootProject.projectDir.parentFile?.parentFile ?: rootProject.projectDir

val prepareEmbeddedRuntimeExecutables by tasks.registering(Sync::class) {
    from("src/main/assets/bin") {
        include("*/xray", "*/obfuscator")
        includeEmptyDirs = false
        eachFile {
            val abi = relativePath.segments.firstOrNull() ?: return@eachFile
            val executableName = name
            relativePath = RelativePath(true, abi, "libnovpn_${executableName}_exec.so")
        }
    }
    into(embeddedRuntimeExecLibsDir)
}

val prepareSecuredAssets by tasks.registering {
    outputs.dir(securedAssetsDir)
    doLast {
        val outputRoot = securedAssetsDir.get().asFile
        if (outputRoot.exists()) {
            outputRoot.deleteRecursively()
        }
        outputRoot.mkdirs()

        fun deriveKey(salt: String): ByteArray {
            return MessageDigest.getInstance("SHA-256")
                .digest(salt.toByteArray(Charsets.UTF_8))
        }

        fun gzip(input: ByteArray): ByteArray {
            val output = ByteArrayOutputStream()
            GZIPOutputStream(output).use { stream ->
                stream.write(input)
            }
            return output.toByteArray()
        }

        fun xor(input: ByteArray, key: ByteArray): ByteArray {
            return ByteArray(input.size) { index ->
                (input[index].toInt() xor key[index % key.size].toInt()).toByte()
            }
        }

        fun writeEncoded(source: File, targetRelativePath: String, salt: String) {
            require(source.exists()) { "Missing secured asset source: ${source.absolutePath}" }
            val raw = source.readBytes()
            val compressed = gzip(raw)
            val obfuscated = xor(compressed, deriveKey(salt))
            val encoded = Base64.getEncoder().encodeToString(obfuscated)
            val target = File(outputRoot, targetRelativePath)
            target.parentFile?.mkdirs()
            target.writeText(encoded + "\n")
        }

        writeEncoded(
            source = File(repoRootDir, "ru app package.txt"),
            targetRelativePath = "catalog/c0.bin",
            salt = "catalog-app-packages-v1"
        )
        writeEncoded(
            source = File(repoRootDir, "ru site list.txt"),
            targetRelativePath = "catalog/c1.bin",
            salt = "catalog-site-list-v1"
        )
        writeEncoded(
            source = file("src/main/secure/bootstrap.json"),
            targetRelativePath = "bootstrap/b0.bin",
            salt = "bootstrap-server-address-v1"
        )
    }
}

android {
    namespace = "com.novpn"
    compileSdk = 35
    ndkVersion = "27.0.12077973"

    defaultConfig {
        applicationId = disguiseAppId
        minSdk = 26
        targetSdk = 35
        versionCode = 1
        versionName = "0.1.0"
        manifestPlaceholders["appLabel"] = disguiseAppName
        ndk {
            abiFilters += listOf("arm64-v8a", "armeabi-v7a", "x86", "x86_64")
        }

        externalNativeBuild {
            cmake {
                cppFlags += "-std=c++17"
            }
        }
    }

    buildTypes {
        release {
            // Produce hardened release artifacts even without a custom keystore.
            signingConfig = signingConfigs.getByName("debug")
            isMinifyEnabled = true
            isShrinkResources = true
            isDebuggable = false
            ndk {
                debugSymbolLevel = "NONE"
            }
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro"
            )
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlinOptions {
        jvmTarget = "17"
    }

    externalNativeBuild {
        cmake {
            path = file("src/main/cpp/CMakeLists.txt")
            buildStagingDirectory = file(".cxx")
        }
    }

    sourceSets.getByName("main").jniLibs.srcDir(embeddedRuntimeExecLibsDir)
    sourceSets.getByName("main").assets.srcDir(securedAssetsDir)

    packaging {
        jniLibs.useLegacyPackaging = true
    }
}

dependencies {
    implementation("androidx.core:core-ktx:1.15.0")
    implementation("androidx.appcompat:appcompat:1.7.0")
    implementation("androidx.activity:activity-ktx:1.10.0")
    implementation("androidx.lifecycle:lifecycle-viewmodel-ktx:2.8.7")
    implementation("androidx.lifecycle:lifecycle-runtime-ktx:2.8.7")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.9.0")
}

tasks.named("preBuild") {
    dependsOn(prepareEmbeddedRuntimeExecutables)
    dependsOn(prepareSecuredAssets)
}
