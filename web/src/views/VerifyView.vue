<script setup>
import { ref, onMounted, onUnmounted } from 'vue';
import { useRoute } from 'vue-router';
import { ShieldCheck, Loader2, CheckCircle, XCircle } from 'lucide-vue-next';
import { getCaptchaConfig, verifyDownload } from '@/services/api';

const route = useRoute();

const filePath = ref('');
const captchaId = ref('');
const isLoading = ref(true);
const isVerifying = ref(false);
const verifyStatus = ref('pending');
const errorMessage = ref('');
const downloadToken = ref('');

let captchaObj = null;

const showCaptcha = () => {
  if (captchaObj) {
    isLoading.value = false;
    verifyStatus.value = 'pending';
    captchaObj.showCaptcha();
  }
};

const verifyCaptcha = async (lotNumber, captchaOutput, passToken, genTime) => {
  isVerifying.value = true;
  verifyStatus.value = 'pending';

  try {
    const response = await verifyDownload(lotNumber, captchaOutput, passToken, genTime, filePath.value);
    downloadToken.value = response.data.download_token;
    verifyStatus.value = 'success';
    isLoading.value = false;

    setTimeout(() => {
      startDownload();
    }, 1000);
  } catch (error) {
    console.error('Verify download error:', error);
    errorMessage.value = error.response?.data?.message || '验证失败，请重试';
    verifyStatus.value = 'error';
    isLoading.value = false;
  } finally {
    isVerifying.value = false;
  }
};

const startDownload = () => {
  const downloadUrl = `/download/${filePath.value}?token=${downloadToken.value}`;
  window.location.href = downloadUrl;
};

const loadCaptchaScript = () => {
  return new Promise((resolve, reject) => {
    if (window.initGeetest4) {
      resolve();
      return;
    }

    const existingScript = document.querySelector('script[src*="gt4.js"]');
    if (existingScript) {
      const checkLoaded = setInterval(() => {
        if (window.initGeetest4) {
          clearInterval(checkLoaded);
          resolve();
        }
      }, 100);
      return;
    }

    const script = document.createElement('script');
    script.src = 'https://static.geetest.com/v4/gt4.js';
    script.async = true;

    script.onload = () => resolve();
    script.onerror = () => reject(new Error('Failed to load Geetest script'));
    document.head.appendChild(script);
  });
};

const initCaptcha = () => {
  isLoading.value = true;
  verifyStatus.value = 'pending';

  window.initGeetest4({
    captchaId: captchaId.value,
    product: 'bind'
  }, (captcha) => {
    captchaObj = captcha;

    captcha.onReady(() => {
      isLoading.value = false;
      captcha.showCaptcha();
    });

    captcha.onSuccess(() => {
      const result = captcha.getValidate();
      console.log('Geetest validate result:', result);
      if (result && result.lot_number) {
        verifyCaptcha(result.lot_number, result.captcha_output, result.pass_token, result.gen_time);
      } else {
        isLoading.value = false;
        verifyStatus.value = 'error';
        errorMessage.value = '验证结果获取失败，请重试';
      }
    });

    captcha.onError((e) => {
      isLoading.value = false;
      verifyStatus.value = 'error';
      errorMessage.value = '验证加载失败: ' + (e.msg || '未知错误');
    });

    captcha.onClose(() => {
      isLoading.value = false;
      verifyStatus.value = 'error';
      errorMessage.value = '用户取消验证';
    });
  });
};

const init = async () => {
  isLoading.value = true;
  filePath.value = route.query.file || '';

  if (!filePath.value) {
    errorMessage.value = '缺少文件参数';
    verifyStatus.value = 'error';
    isLoading.value = false;
    return;
  }

  try {
    const [response] = await Promise.all([
      getCaptchaConfig(),
      loadCaptchaScript()
    ]);
    
    captchaId.value = response.data.app_id;
    console.log('Config loaded:', { enabled: response.data.enabled, captchaId: captchaId.value });

    if (!response.data.enabled) {
      window.location.href = `/download/${filePath.value}`;
      return;
    }

    initCaptcha();
  } catch (error) {
    console.error('Init error:', error);
    errorMessage.value = '加载配置失败: ' + error.message;
    verifyStatus.value = 'error';
    isLoading.value = false;
  }
};

onMounted(() => {
  init();
});

onUnmounted(() => {
  if (captchaObj) {
    captchaObj.destroy();
  }
});
</script>

<template>
  <div class="verify-page">
    <div class="verify-container">
      <div class="verify-card">
        <div class="verify-header">
          <ShieldCheck class="h-12 w-12 text-primary" />
          <h1>安全验证</h1>
          <p>请完成验证后开始下载</p>
        </div>

        <div class="verify-content">
          <div v-if="isLoading && verifyStatus !== 'error'" class="verify-loading">
            <Loader2 class="h-8 w-8 animate-spin" />
            <span>正在加载验证...</span>
          </div>

          <div v-else-if="verifyStatus === 'success'" class="verify-success">
            <CheckCircle class="h-16 w-16 text-green-500" />
            <span>验证成功，正在开始下载...</span>
          </div>

          <div v-else-if="verifyStatus === 'error'" class="verify-error">
            <XCircle class="h-16 w-16 text-red-500" />
            <span>{{ errorMessage }}</span>
            <button @click="showCaptcha" class="retry-btn">重新验证</button>
          </div>
        </div>

        <div class="verify-footer">
          <p class="file-path" v-if="filePath">
            文件: {{ filePath.split('/').pop() }}
          </p>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.verify-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #f5f7fa 0%, #e4e8ec 100%);
  padding: 20px;
}

@media (prefers-color-scheme: dark) {
  .verify-page {
    background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%);
  }
}

.verify-container {
  width: 100%;
  max-width: 420px;
}

.verify-card {
  background: #ffffff;
  border-radius: 16px;
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.1);
  overflow: hidden;
}

@media (prefers-color-scheme: dark) {
  .verify-card {
    background: #1f2937;
  }
}

.verify-header {
  text-align: center;
  padding: 32px 24px 24px;
  border-bottom: 1px solid #e5e7eb;
}

@media (prefers-color-scheme: dark) {
  .verify-header {
    border-bottom-color: #374151;
  }
}

.verify-header h1 {
  margin: 16px 0 8px;
  font-size: 24px;
  font-weight: 600;
  color: #111827;
}

@media (prefers-color-scheme: dark) {
  .verify-header h1 {
    color: #f3f4f6;
  }
}

.verify-header p {
  color: #6b7280;
  font-size: 14px;
}

.verify-content {
  padding: 32px 24px;
  min-height: 200px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.verify-loading,
.verify-success,
.verify-error {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
  text-align: center;
}

.verify-success span,
.verify-error span {
  color: #6b7280;
  font-size: 14px;
}

.retry-btn {
  margin-top: 8px;
  padding: 8px 24px;
  background: #3b82f6;
  color: white;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  font-size: 14px;
}

.retry-btn:hover {
  opacity: 0.9;
}

.verify-footer {
  padding: 16px 24px;
  border-top: 1px solid #e5e7eb;
  text-align: center;
}

@media (prefers-color-scheme: dark) {
  .verify-footer {
    border-top-color: #374151;
  }
}

.file-path {
  font-size: 12px;
  color: #6b7280;
  word-break: break-all;
}
</style>
