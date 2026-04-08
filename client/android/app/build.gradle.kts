import org.gradle.api.file.RelativePath
import org.gradle.api.tasks.Sync

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

val disguiseAppId = providers.gradleProperty("novpnAppId").orNull
    ?: System.getenv("NOVPN_APP_ID")
    ?: "safety.turtle"
val disguiseAppName = providers.gradleProperty("novpnAppName").orNull
    ?: System.getenv("NOVPN_APP_NAME")
    ?: "Safaty Turtle"

val embeddedRuntimeExecLibsDir = layout.buildDirectory.dir("generated/embeddedRuntimeExecJniLibs")

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
            isMinifyEnabled = false
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
        }
    }

    sourceSets.getByName("main").jniLibs.srcDir(embeddedRuntimeExecLibsDir)

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
}
