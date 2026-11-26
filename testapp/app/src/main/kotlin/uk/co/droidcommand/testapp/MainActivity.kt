package uk.co.droidcommand.testapp

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.testTagsAsResourceId
import androidx.compose.ui.unit.dp

/** Stable test tags. They are exposed as resource-ids so uiautomator dumps can find them. */
object Tags {
    const val TITLE = "title"
    const val COUNTER = "counter"
    const val INCREMENT = "increment"
    const val INPUT = "input"
    const val ECHO = "echo"
    const val NAV_NEXT = "nav_next"
    const val NAV_BACK = "nav_back"
    const val SCREEN_TWO = "screen_two"
}

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            MaterialTheme {
                Surface(
                    modifier = Modifier
                        .fillMaxSize()
                        .semantics { testTagsAsResourceId = true },
                ) {
                    App()
                }
            }
        }
    }
}

@Composable
fun App() {
    var onSecond by rememberSaveable { mutableStateOf(false) }
    if (onSecond) SecondScreen { onSecond = false } else HomeScreen { onSecond = true }
}

@Composable
fun HomeScreen(onNext: () -> Unit) {
    var count by rememberSaveable { mutableIntStateOf(0) }
    var text by rememberSaveable { mutableStateOf("") }
    Column(
        modifier = Modifier.padding(24.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Text("DroidCommand Test App", Modifier.testTag(Tags.TITLE), style = MaterialTheme.typography.headlineSmall)
        Text("Count: $count", Modifier.testTag(Tags.COUNTER))
        Button(onClick = { count++ }, modifier = Modifier.testTag(Tags.INCREMENT)) { Text("Increment") }
        OutlinedTextField(
            value = text,
            onValueChange = { text = it },
            label = { Text("Input") },
            modifier = Modifier.testTag(Tags.INPUT),
        )
        Text("Echo: $text", Modifier.testTag(Tags.ECHO))
        Button(onClick = onNext, modifier = Modifier.testTag(Tags.NAV_NEXT)) { Text("Next screen") }
    }
}

@Composable
fun SecondScreen(onBack: () -> Unit) {
    Column(
        modifier = Modifier.padding(24.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Text("Second screen", Modifier.testTag(Tags.SCREEN_TWO), style = MaterialTheme.typography.headlineSmall)
        Button(onClick = onBack, modifier = Modifier.testTag(Tags.NAV_BACK)) { Text("Back") }
    }
}
